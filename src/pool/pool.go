// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package pool

import (
	. "config"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"goburst/burstmath"
	. "logger"
	. "modelx"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
	"wallet"

	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
)

const (
	submitBefore           = 30 * time.Second
	nonceSubmissionRetries = 3
)

type Pool struct {
	modelx                 *Modelx
	walletHandler          wallet.WalletHandler
	miningInfoJSON         atomic.Value
	nonceSubmissions       chan NonceSubmission
	newBlocks              chan Block
	roundInfo              atomic.Value
	deadlineRequestHandler *burstmath.DeadlineRequestHandler
}

type roundInfo struct {
	scoop      uint32
	baseTarget uint64
	height     uint64
	genSig     []byte
}

func NewPool(modelx *Modelx, walletHandler wallet.WalletHandler) *Pool {
	miningInfo, err := walletHandler.GetMiningInfo()
	if err != nil {
		Logger.Fatal("Error getting mining info")
	}

	miningInfoBytes, err := jsonFromMiningInfo(miningInfo)
	if err != nil {
		Logger.Fatal("Error encoding mining info")
	}

	pool := &Pool{
		walletHandler:          walletHandler,
		modelx:                 modelx,
		nonceSubmissions:       make(chan NonceSubmission),
		newBlocks:              make(chan Block),
		deadlineRequestHandler: burstmath.NewDeadlineRequestHandler(runtime.NumCPU())}

	pool.miningInfoJSON.Store(miningInfoBytes)

	currentBlock := Cache.CurrentBlock()
	pool.updateRoundInfo(&currentBlock)

	go pool.checkAndAddNewBlockJob()
	go pool.forge(currentBlock)

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
	pool.roundInfo.Store(roundInfo{
		scoop:      block.Scoop,
		baseTarget: block.BaseTarget,
		height:     block.Height,
		genSig:     block.GenerationSignatureBytes})
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
			if bestNonceSubmission.Deadline <= nonceSubmission.Deadline ||
				currentBlock.Height != nonceSubmission.Height {
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
	walletDeadline, err := pool.walletHandler.SubmitNonce(nonceSubmission.Nonce, nonceSubmission.MinerID)
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
	miningInfo, err := pool.walletHandler.GetMiningInfo()
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
		pool.miningInfoJSON.Store(miningInfoBytes)

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
	ri := pool.roundInfo.Load().(roundInfo)
	requestLogger := RequestLogger(req)

	if minerHeight, err := strconv.ParseUint(req.Form.Get("blockheight"), 10, 64); err == nil {
		if minerHeight != ri.height {
			requestLogger.Warn("Miner submitted on invalid height",
				zap.Uint64("got", minerHeight), zap.Uint64("expected", ri.height))
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
	deadlineReq := burstmath.NewCalcDeadlineRequest(accountID, nonce, ri.baseTarget, ri.scoop, ri.genSig,
		ri.height >= Cfg.PoC2StartHeight)
	deadline := pool.deadlineRequestHandler.CalcDeadline(deadlineReq)

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

	err = pool.modelx.UpdateOrCreateNonceSubmission(miner, ri.height, deadline, nonce, ri.baseTarget)
	if err != nil {
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
		Height:   ri.height}
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
	// ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	// RequestLogger(req).Info("incoming request", zap.String("ip", ip), zap.String("uri", req.RequestURI),
	// 	zap.String("user-agent", req.UserAgent()))
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
				w.Write(pool.miningInfoJSON.Load().([]byte))
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
