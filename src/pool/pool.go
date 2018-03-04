// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package pool

import (
	. "config"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	. "logger"
	. "modelx"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"util"
	"wallet"

	"github.com/klauspost/cpuid"
	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
)

const (
	AVX2Parallel           = 8
	SSE4Parallel           = 4
	submitBefore           = 30 * time.Second
	nonceSubmissionRetries = 3
)

type Pool struct {
	modelx            *Modelx
	wallet            wallet.Wallet
	miningInfoJSONMu  sync.RWMutex
	miningInfoJSON    []byte
	nonceSubmissions  chan NonceSubmission
	newBlocks         chan Block
	calcDeadlineReqs  chan calcDeadlineReq
	currentScoop      uint32
	currentBaseTarget uint64
	currentHeight     uint64
	currentGenSig     atomic.Value
}

type calcDeadlineReq struct {
	accountID uint64
	nonce     uint64
	deadline  chan uint64
}

func NewPool(modelx *Modelx, wallet wallet.Wallet) *Pool {
	miningInfo, err := wallet.GetMiningInfo()
	if err != nil {
		Logger.Fatal("Error getting mining info")
	}

	miningInfoBytes, err := jsonFromMiningInfo(miningInfo)
	if err != nil {
		Logger.Fatal("Error encoding mining info")
	}

	pool := &Pool{
		wallet:           wallet,
		modelx:           modelx,
		miningInfoJSON:   miningInfoBytes,
		miningInfoJSONMu: sync.RWMutex{},
		nonceSubmissions: make(chan NonceSubmission),
		newBlocks:        make(chan Block),
		calcDeadlineReqs: make(chan calcDeadlineReq)}

	currentBlock := Cache.CurrentBlock()
	pool.updateRoundInfo(&currentBlock)

	go pool.checkAndAddNewBlockJob()
	go pool.forge(currentBlock)

	if cpuid.CPU.AVX2() {
		Logger.Info("using avx2")
		go pool.collectDeadlineReqsAVX2()
	} else if cpuid.CPU.SSE4() {
		Logger.Info("using sse4")
		go pool.collectDeadlineReqsSSE4()
	} else {
		Logger.Fatal("avx2 and sse4 not available")
	}

	return pool
}

func jsonFromMiningInfo(miningInfo *wallet.MiningInfo) ([]byte, error) {
	miningInfoBytes, err := json.Marshal(map[string]interface{}{
		"baseTarget":          miningInfo.BaseTarget,
		"generationSignature": miningInfo.GenerationSignature,
		"height":              miningInfo.Height,
		"targetDeadline":      Cfg.DeadlineLimit})

	if err != nil {
		Logger.Error("error encoding miningInfo to json", zap.Error(err))
		return nil, err
	}

	return miningInfoBytes, nil
}

func (pool *Pool) updateRoundInfo(block *Block) {
	atomic.StoreUint32(&pool.currentScoop, block.Scoop)
	atomic.StoreUint64(&pool.currentBaseTarget, block.BaseTarget)
	atomic.StoreUint64(&pool.currentHeight, block.Height)
	pool.currentGenSig.Store(block.GenerationSignatureBytes)
}

func (pool *Pool) collectDeadlineReqsSSE4() {
	var start <-chan time.Time

	var reqs [SSE4Parallel]calcDeadlineReq
	var pending int
	for {
		select {
		case reqs[pending] = <-pool.calcDeadlineReqs:
			if pending == 0 {
				start = time.After(2 * time.Second)
			} else if pending == SSE4Parallel-1 {
				go pool.processReqsSSE4(reqs, pending+1)
				pending = 0
				continue
			}
			pending++
		case <-start:
			go pool.processReqsSSE4(reqs, pending+1)
			pending = 0
		}
	}
}

func (pool *Pool) processReqsSSE4(reqs [SSE4Parallel]calcDeadlineReq, total int) {
	var dls [SSE4Parallel]uint64
	dls[0], dls[1], dls[2], dls[3] = util.CalculateDeadlinesSSE4(
		atomic.LoadUint32(&pool.currentScoop), atomic.LoadUint64(&pool.currentBaseTarget),
		pool.currentGenSig.Load().([]byte), atomic.LoadUint64(&pool.currentHeight) >= Cfg.PoC2StartHeight,

		reqs[0].accountID, reqs[1].accountID, reqs[2].accountID, reqs[3].accountID,

		reqs[0].nonce, reqs[1].nonce, reqs[2].nonce, reqs[3].nonce)

	for i := 0; i < total; i++ {
		reqs[i].deadline <- dls[i]
	}
}

func (pool *Pool) collectDeadlineReqsAVX2() {
	var start <-chan time.Time

	var reqs [AVX2Parallel]calcDeadlineReq
	var pending int
	for {
		select {
		case reqs[pending] = <-pool.calcDeadlineReqs:
			if pending == 0 {
				start = time.After(2 * time.Second)
			} else if pending == AVX2Parallel-1 {
				go pool.processReqsAVX2(reqs, pending+1)
				pending = 0
				continue
			}
			pending++
		case <-start:
			go pool.processReqsAVX2(reqs, pending+1)
			pending = 0
		}
	}
}

func (pool *Pool) processReqsAVX2(reqs [AVX2Parallel]calcDeadlineReq, total int) {
	var dls [AVX2Parallel]uint64
	dls[0], dls[1], dls[2], dls[3], dls[4], dls[5], dls[6], dls[7] = util.CalculateDeadlinesAVX2(
		atomic.LoadUint32(&pool.currentScoop), atomic.LoadUint64(&pool.currentBaseTarget),
		pool.currentGenSig.Load().([]byte), atomic.LoadUint64(&pool.currentHeight) >= Cfg.PoC2StartHeight,

		reqs[0].accountID, reqs[1].accountID, reqs[2].accountID, reqs[3].accountID,
		reqs[4].accountID, reqs[5].accountID, reqs[6].accountID, reqs[7].accountID,

		reqs[0].nonce, reqs[1].nonce, reqs[2].nonce, reqs[3].nonce,
		reqs[4].nonce, reqs[5].nonce, reqs[6].nonce, reqs[7].nonce)

	for i := 0; i < total; i++ {
		reqs[i].deadline <- dls[i]
	}
}

func (pool *Pool) forge(currentBlock Block) {
	var bestNonceSubmission *NonceSubmission
	maxTime := time.Duration(1<<63 - 1)

	var after <-chan time.Time
	updateSubmitTimer := func(deadline uint64) {
		submitDelay := time.Duration(deadline)*time.Second - time.Since(currentBlock.Created) - submitBefore
		Logger.Info("planning submitNonce", zap.Duration("delay", submitDelay))
		after = time.After(submitDelay)
	}

	if currentBlock.BestNonceSubmissionID.Valid {
		var err error
		bestNonceSubmission, err = pool.modelx.GetBestNonceSubmissionOnBlock(currentBlock.Height)
		if err != nil {
			Logger.Fatal("error getting best nonce submission", zap.Error(err))
		}
		Cache.StoreBestNonceSubmission(*bestNonceSubmission)
		updateSubmitTimer(bestNonceSubmission.Deadline)
	} else {
		// if we don't have a current time wait forever and don't submit anything
		bestNonceSubmission = &NonceSubmission{Deadline: ^uint64(0)}
		after = time.After(maxTime)
	}

	for {
		select {
		case nonceSubmission := <-pool.nonceSubmissions:
			if bestNonceSubmission.Deadline < nonceSubmission.Deadline {
				continue
			}
			Logger.Info("new best deadline", zap.Uint64("deadline", nonceSubmission.Deadline))
			bestNonceSubmission = &nonceSubmission
			pool.modelx.UpdateBestSubmission(nonceSubmission.MinerID, nonceSubmission.Height)
			Cache.StoreBestNonceSubmission(*bestNonceSubmission)
			updateSubmitTimer(bestNonceSubmission.Deadline)
		case <-after:
			pool.submitNonce(bestNonceSubmission)
		case currentBlock = <-pool.newBlocks:
			after = time.After(maxTime)
			bestNonceSubmission.Deadline = ^uint64(0)
			bestNonceSubmission.MinerID = 0
			bestNonceSubmission.Nonce = 0
		}
	}
}

func (pool *Pool) submitNonce(nonceSubmission *NonceSubmission) {
	Logger.Info("submitting best nonce")
	tries := 0
RETRY:
	walletDeadline, err := pool.wallet.SubmitNonce(nonceSubmission.Nonce, nonceSubmission.MinerID)
	if err != nil {
		if tries < nonceSubmissionRetries {
			tries++
			Logger.Error("Submitting nonce failed", zap.Int("try", tries),
				zap.Int("max-tries", nonceSubmissionRetries))
			goto RETRY
		}
		Logger.Error("Submitting nonce failed - finally")
	} else if walletDeadline != nonceSubmission.Deadline {
		Logger.Error("Pool deadline doesn't match wallet's deadline",
			zap.Uint64("deadline-pool", nonceSubmission.Deadline),
			zap.Uint64("deadline-wallet", walletDeadline))
	}
}

func (pool *Pool) checkAndAddNewBlock() {
	miningInfo, err := pool.wallet.GetMiningInfo()
	if err != nil {
		return
	}

	oldBlock := Cache.CurrentBlock()
	if oldBlock.Height < miningInfo.Height {
		Logger.Info("got new Block with height", zap.Uint64("height", miningInfo.Height))

		newBlock, err := pool.modelx.NewBlock(miningInfo.BaseTarget, miningInfo.GenerationSignature,
			miningInfo.Height)

		if err != nil {
			Logger.Error("creating new block", zap.Error(err))
			return
		}

		miningInfoBytes, err := jsonFromMiningInfo(miningInfo)

		if err != nil {
			return
		}

		pool.updateRoundInfo(&newBlock)

		pool.miningInfoJSONMu.Lock()
		pool.miningInfoJSON = miningInfoBytes
		pool.miningInfoJSONMu.Unlock()

		// for forging
		pool.newBlocks <- newBlock
	}
}

func (pool *Pool) checkAndAddNewBlockJob() {
	pool.checkAndAddNewBlock()
	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		pool.checkAndAddNewBlock()
	}
}

func (pool *Pool) jobs() {
	payTicker := time.NewTicker(10 * time.Minute)
	rereadMinerNamesTicker := time.NewTicker(12 * time.Hour)
	cleanDBTicker := time.NewTicker(24 * time.Hour)

	for {
		select {
		case <-payTicker.C:
			pool.modelx.RewardBlocks()
			pool.modelx.Payout()
		case <-rereadMinerNamesTicker.C:
			pool.modelx.RereadMinerNames()
		case <-cleanDBTicker.C:
			pool.modelx.CleanDB()
		}
	}
}

func formatJSONError(errorCode int64, errorMsg string) []uint8 {
	bytes, _ := json.Marshal(map[string]string{
		"errorCode":        strconv.FormatInt(errorCode, 10),
		"errorDescription": errorMsg})
	return bytes
}

func (pool *Pool) processSubmitNonceRequest(w http.ResponseWriter, req *http.Request) {
	currentBlock := Cache.CurrentBlock()
	requestLogger := RequestLogger(req)

	if minerHeight, err := strconv.ParseUint(req.Form.Get("blockheight"), 10, 64); err == nil {
		if minerHeight != currentBlock.Height {
			requestLogger.Warn("Miner submitted on invalid height",
				zap.Uint64("got", minerHeight), zap.Uint64("expected", currentBlock.Height))
			w.WriteHeader(http.StatusBadRequest)
			w.Write(formatJSONError(1005, "Submitted on wrong height"))
			return
		}
	}

	// Extract params and check for errors
	nonceStr := req.Form.Get("nonce")
	nonce, err := strconv.ParseUint(nonceStr, 10, 64)
	if err != nil {
		requestLogger.Warn("malformed nonce", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write(formatJSONError(1012, "submitNonce request has bad 'nonce' parameter - should be uint64"))
		return
	}

	accountIDStr := req.Form.Get("accountId")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 64)
	if err != nil || accountID == 0 {
		requestLogger.Warn("malformed accountId", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write(formatJSONError(1013, "submitNonce request has bad 'accountId' parameter - should be uint64"))
		return
	}

	requestLogger.Info("processing formal valid request", zap.Uint64("accountID", accountID),
		zap.Uint64("nonce", nonce))

	// Check if the reward recepient is correct and cache it for this round
	correctRewardRecepient, err := pool.modelx.IsPoolRewardRecipient(accountID)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write(formatJSONError(1014, "Account's reward recipient couldn't be determined"))
		return
	} else if !correctRewardRecepient {
		w.WriteHeader(http.StatusForbidden)
		w.Write(formatJSONError(1004, "Account's reward recipient doesn't match the pool's"))
		requestLogger.Warn("reward recipient doesn't match pools", zap.Uint64("accountID", accountID))
		return
	}
	requestLogger.Info("valid reward recipient")

	// Create a new miner or get it from cache
	miner := pool.modelx.FirstOrCreateMiner(accountID)
	if miner == nil {
		// most likely wrong reward recipient, can also be error in db
		requestLogger.Warn("invalid reward recipient", zap.Error(err))
		w.WriteHeader(http.StatusForbidden)
		w.Write(formatJSONError(1004, "Account's reward recipient doesn't match the pool's"))
		return
	}

	// Calculate deadline and check against limit
	deadlineReq := calcDeadlineReq{
		accountID: accountID,
		nonce:     nonce,
		deadline:  make(chan uint64, 1)}
	pool.calcDeadlineReqs <- deadlineReq
	deadline := <-deadlineReq.deadline

	if Cfg.DeadlineLimit != 0 && deadline > Cfg.DeadlineLimit {
		requestLogger.Warn("calculated deadline exceeds pool limit", zap.Uint64("got", deadline),
			zap.Uint64("expected-max", Cfg.DeadlineLimit))
		w.WriteHeader(http.StatusBadRequest)
		w.Write(formatJSONError(1008, "deadline exceeds deadline limit of the pool"))
		return
	}

	requestLogger.Info("valid deadline", zap.Uint64("deadline", deadline))

	deadlineResp, _ := json.Marshal(wallet.NonceInfoResponse{
		Deadline: deadline,
		Result:   "success"})
	w.Write(deadlineResp)

	miner.Lock()
	defer miner.Unlock()

	if err := pool.modelx.UpdateOrCreateNonceSubmission(miner, &currentBlock, deadline, nonce); err != nil {
		requestLogger.Error("updating deadline failed", zap.Error(err))
		return
	}

	// Check if this is the best deadline and submit it to the wallet as soon as it comes close
	nonceSubmission := NonceSubmission{
		MinerID:  accountID,
		Name:     miner.Name,
		Address:  miner.Address,
		Deadline: deadline,
		Nonce:    nonce,
		Height:   currentBlock.Height}
	pool.nonceSubmissions <- nonceSubmission
}

func rateLimitDeniedHandler(w http.ResponseWriter, req *http.Request) {
	logIncomingRequest(req)
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	RequestLogger(req).Info("rate limit exceeded", zap.String("ip", ip), zap.String("uri", req.RequestURI),
		zap.String("user-agent", req.UserAgent()))
	http.Error(w, "limit exceeded", 429)
}

func generateLimiterKey(req *http.Request) string {
	req.ParseForm()
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	return ip + req.Form.Get("requestType")
}

func logIncomingRequest(req *http.Request) {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	RequestLogger(req).Info("incoming request", zap.String("ip", ip), zap.String("uri", req.RequestURI),
		zap.String("user-agent", req.UserAgent()))
}

func (pool *Pool) serve() {
	store, err := memstore.New(65536)
	if err != nil {
		Logger.Fatal("", zap.Error(err))
	}

	quota := throttled.RateQuota{
		MaxRate:  throttled.PerSec(Cfg.AllowRequestsPerSecond),
		MaxBurst: 2}

	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		Logger.Fatal("", zap.Error(err))
	}

	httpRateLimiter := throttled.HTTPRateLimiter{
		RateLimiter:   rateLimiter,
		VaryBy:        &throttled.VaryBy{Custom: generateLimiterKey},
		DeniedHandler: http.Handler(http.HandlerFunc(rateLimitDeniedHandler))}

	http.Handle("/burst", httpRateLimiter.RateLimit(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			logIncomingRequest(req)

			switch req.Form.Get("requestType") {
			case "getMiningInfo":
				pool.miningInfoJSONMu.RLock()
				w.Write(pool.miningInfoJSON)
				pool.miningInfoJSONMu.RUnlock()
			case "submitNonce":
				pool.processSubmitNonceRequest(w, req)
			}
		})))
	http.ListenAndServe(fmt.Sprintf("%s:%d", Cfg.PoolListenAddress, Cfg.PoolPort), nil)
}

func (pool *Pool) Run() {
	go pool.jobs()
	go pool.serve()
}
