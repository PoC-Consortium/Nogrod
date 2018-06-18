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
	"nodecom"
	"runtime"
	"strconv"
	"time"
	"wallethandler"

	"github.com/throttled/throttled"
	"github.com/throttled/throttled/store/memstore"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

const (
	submitBefore           = 30 * time.Second
	nonceSubmissionRetries = 3
)

type Pool struct {
	modelx                 *Modelx
	walletHandler          wallethandler.WalletHandler
	nonceSubmissions       chan *NonceSubmission
	deadlineRequestHandler *burstmath.DeadlineRequestHandler
}

type nodeServer struct {
	modelx           *Modelx
	nonceSubmissions chan *NonceSubmission
}

func NewPool(modelx *Modelx, walletHandler wallethandler.WalletHandler) *Pool {
	pool := &Pool{
		walletHandler:          walletHandler,
		modelx:                 modelx,
		nonceSubmissions:       make(chan *NonceSubmission),
		deadlineRequestHandler: burstmath.NewDeadlineRequestHandler(runtime.NumCPU())}

	currentBlock := Cache.CurrentBlock()

	go pool.checkAndAddNewBlockJob()
	go pool.forge(currentBlock)

	return pool
}

func (pool *Pool) forge(currentBlock Block) {
	var bestNonceSubmission *NonceSubmission
	maxTime := time.Duration(1<<63 - 1)

	var after <-chan time.Time
	updateSubmitTimer := func(deadline uint64, roundStart time.Time) {
		due := time.Duration(deadline)*time.Second - time.Since(roundStart) - submitBefore
		Logger.Info("planning submitNonce", zap.Duration("delay", due))
		after = time.After(due)
	}

	if currentBlock.BestNonceSubmissionID.Valid {
		var err error
		bestNonceSubmission, err = pool.modelx.GetBestNonceSubmissionOnBlock(currentBlock.Height)
		if err != nil {
			Logger.Fatal("error getting best nonce submission", zap.Error(err))
		}
		Cache.StoreBestNonceSubmission(*bestNonceSubmission)
		updateSubmitTimer(bestNonceSubmission.Deadline, bestNonceSubmission.RoundStart)
	} else {
		// if we don't have a current time wait forever and don't submit anything
		bestNonceSubmission = &NonceSubmission{Deadline: ^uint64(0)}
		after = time.After(maxTime)
	}

	for {
		select {
		case nonceSubmission := <-pool.nonceSubmissions:
			// ignore old blocks
			if nonceSubmission.Height < bestNonceSubmission.Height {
				continue
			}

			// ignore worse deadlines
			if nonceSubmission.Height == bestNonceSubmission.Height &&
				nonceSubmission.Deadline >= bestNonceSubmission.Deadline {
				continue
			}

			Logger.Info("new best deadline", zap.Uint64("deadline", nonceSubmission.Deadline))
			bestNonceSubmission = nonceSubmission
			pool.modelx.UpdateBestSubmission(nonceSubmission.MinerID, nonceSubmission.Height)
			Cache.StoreBestNonceSubmission(*bestNonceSubmission)

			updateSubmitTimer(nonceSubmission.Deadline, nonceSubmission.RoundStart)
		case <-after:
			pool.submitNonce(bestNonceSubmission)
		}
	}
}

func (pool *Pool) submitNonce(nonceSubmission *NonceSubmission) {
	Logger.Info("submitting best nonce")
	for try := 0; try < nonceSubmissionRetries; try++ {
		err := pool.walletHandler.SubmitNonce(nonceSubmission.Nonce, nonceSubmission.MinerID,
			nonceSubmission.Deadline)
		if err == nil {
			return
		}
		Logger.Error("Submitting nonce failed", zap.Int("try", try),
			zap.Int("max-tries", nonceSubmissionRetries))
	}
	Logger.Error("Submitting nonce failed", zap.Int("max-tries", nonceSubmissionRetries))
}

func (pool *Pool) checkAndAddNewBlock() {
	miningInfo, err := pool.walletHandler.GetMiningInfo()
	if err != nil {
		return
	}

	oldBlock := Cache.CurrentBlock()
	if oldBlock.Height < miningInfo.Height {
		Logger.Info("got new Block with height", zap.Uint64("height", miningInfo.Height))

		err := pool.modelx.NewBlock(miningInfo.BaseTarget, miningInfo.GenerationSignature,
			miningInfo.Height)

		if err != nil {
			Logger.Error("creating new block", zap.Error(err))
			return
		}

		if err != nil {
			return
		}
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
	ri := Cache.GetRoundInfo()
	requestLogger := RequestLogger(req)

	if minerHeight, err := strconv.ParseUint(req.Form.Get("blockheight"), 10, 64); err == nil {
		if minerHeight != ri.Height {
			requestLogger.Warn("Miner submitted on invalid height",
				zap.Uint64("got", minerHeight), zap.Uint64("expected", ri.Height))
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
	deadlineReq := burstmath.NewCalcDeadlineRequest(accountID, nonce, ri.BaseTarget, ri.Scoop, ri.GenSig)
	deadline := pool.deadlineRequestHandler.CalcDeadline(deadlineReq)

	if Cfg.DeadlineLimit != 0 && deadline > Cfg.DeadlineLimit {
		requestLogger.Warn("calculated deadline exceeds pool limit", zap.Uint64("got", deadline),
			zap.Uint64("expected-max", Cfg.DeadlineLimit))
		w.WriteHeader(http.StatusBadRequest)
		w.Write(formatJSONError(1008, "deadline exceeds deadline limit of the pool"))
		return
	}

	requestLogger.Info("valid deadline", zap.Uint64("deadline", deadline))

	w.Write([]byte(fmt.Sprintf("{\"deadline\":%d,\"result\":\"success\"}", deadline)))

	err = pool.modelx.UpdateOrCreateNonceSubmission(miner, ri.Height, deadline, nonce, ri.BaseTarget, "")
	if err != nil {
		requestLogger.Error("updating deadline failed", zap.Error(err))
		return
	}

	// Check if this is the best deadline and submit it to the wallet as soon as it comes close
	nonceSubmission := NonceSubmission{
		MinerID:    accountID,
		Name:       miner.Name,
		Address:    miner.Address,
		Deadline:   deadline,
		Nonce:      nonce,
		RoundStart: ri.RoundStart,
		Height:     ri.Height}
	pool.nonceSubmissions <- &nonceSubmission
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
				w.Write(Cache.GetMiningInfoJSON())
			case "submitNonce":
				pool.processSubmitNonceRequest(w, req)
			}
		})))
	http.ListenAndServe(fmt.Sprintf("%s:%d", Cfg.PoolListenAddress, Cfg.PoolPort), nil)
}

func (s *nodeServer) SubmitNonce(ctx context.Context, msg *nodecom.SubmitNonceRequest) (*nodecom.SubmitNonceReply,
	error) {
	ri := Cache.GetRoundInfo()
	m := s.modelx.FirstOrCreateMiner(msg.AccountID)
	err := s.modelx.UpdateOrCreateNonceSubmission(m, msg.BlockHeight, msg.Deadline, msg.Nonce, msg.BaseTarget,
		msg.GenSig)
	if err != nil {
		return nil, err
	}

	// TODO: probably cache best deadline of roundinfo, there might be too much going
	// through that channel after some time
	if ri.Height <= msg.BlockHeight {
		ri := Cache.GetRoundInfo()
		// TODO: we need to get the round start of exactly the block on which
		// was submitted
		nonceSubmission := NonceSubmission{
			MinerID:    msg.AccountID,
			Name:       m.Name,
			Address:    m.Address,
			Deadline:   msg.Deadline,
			Nonce:      msg.Nonce,
			RoundStart: ri.RoundStart,
			Height:     ri.Height}
		s.nonceSubmissions <- &nonceSubmission
	}
	return &nodecom.SubmitNonceReply{}, nil
}

func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	t := ctx.Value("token")
	if t == nil {
		return nil, grpc.Errorf(codes.Unauthenticated, "incorrect access token")
	}
	token := t.(string)
	if token != "valid-token" {
		return nil, grpc.Errorf(codes.Unauthenticated, "incorrect access token")
	}

	return handler(ctx, req)
}

func (pool *Pool) serveNode() {
	if Cfg.NodePort == 0 {
		return
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", Cfg.NodeListenAddress, Cfg.NodePort))
	if err != nil {
		Logger.Fatal("failed to listen", zap.Error(err))
	}

	var s *grpc.Server
	var creds credentials.TransportCredentials
	if Cfg.NodeComCert != "" {
		creds, err = credentials.NewClientTLSFromFile(Cfg.NodeComCert, "")
		if err != nil {
			Logger.Fatal("create credentials", zap.Error(err))
		}
		s = grpc.NewServer(
			grpc.Creds(creds),
			grpc.UnaryInterceptor(authInterceptor),
		)
	} else {
		s = grpc.NewServer()
	}

	nodecom.RegisterNodeComServer(s, &nodeServer{
		modelx:           pool.modelx,
		nonceSubmissions: pool.nonceSubmissions})
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		Logger.Fatal("failed to server", zap.Error(err))
	}
}

func (pool *Pool) Run() {
	go pool.jobs()
	go pool.serve()
	go pool.serveNode()
}
