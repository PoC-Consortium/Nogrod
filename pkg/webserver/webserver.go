// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/PoC-Consortium/goburstpool/pkg/api"
	"github.com/PoC-Consortium/goburstpool/pkg/burstmath"
	. "github.com/PoC-Consortium/goburstpool/pkg/config"
	. "github.com/PoC-Consortium/goburstpool/pkg/logger"
	"github.com/PoC-Consortium/goburstpool/pkg/modelx"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var errUnknownAccount = errors.New("unknown account")
var errSendingFailed = errors.New("sending through websocket failed")

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	bestShares        = 10
	netDiffBlockRange = 360
)

type WebServer struct {
	modelx *modelx.Modelx

	clients         map[*Client]bool
	newClients      chan *Client
	finishedClients chan *Client

	templates     *template.Template
	blockInfo     atomic.Value
	poolStatsInfo atomic.Value
	upgrader      websocket.Upgrader

	blockUpdates chan *api.BlockInfo
	shareUpdates chan []*Share

	minerInfosMu sync.RWMutex
	minerInfos   []*MinerInfo

	wonBlocksMu sync.RWMutex
	wonBlocks   []modelx.WonBlock

	netDiff atomic.Value
}

type Share struct {
	Name string  `json:"name"`
	Val  float64 `json:"val"`
}

type MinerInfo struct {
	ID                    uint64
	Name                  string
	Address               string
	Pending               float64
	HistoricalShare       float64
	NConf                 int
	LastActiveBlockHeight uint64
	Capacity              float64
	Deadline              uint64
}

type IndexInfo struct {
	Cfg     *Config
	NetDiff float64
}

func NewWebServer(m *modelx.Modelx) *WebServer {
	webServer := &WebServer{
		modelx:          m,
		newClients:      make(chan *Client),
		finishedClients: make(chan *Client),
		clients:         make(map[*Client]bool),
		templates:       &template.Template{},
		blockUpdates:    make(chan *api.BlockInfo),
		shareUpdates:    make(chan []*Share)}

	currentBlock := modelx.Cache.CurrentBlock()
	created, _ := currentBlock.Created.MarshalText()
	webServer.blockInfo.Store(api.BlockInfo{
		Height:              currentBlock.Height,
		BaseTarget:          currentBlock.BaseTarget,
		Scoop:               currentBlock.Scoop,
		GenerationSignature: currentBlock.GenerationSignature,
		Created:             string(created)})

	webServer.loadTemplaes()

	return webServer
}

func (webServer *WebServer) loadTemplaes() {
	templates, err := ioutil.ReadDir("./web/templates")
	if err != nil {
		Logger.Fatal("reading templates failed", zap.Error(err))
		panic("reading templates failed")
	}

	var templateFiles []string
	for _, template := range templates {
		filename := template.Name()
		templateFiles = append(templateFiles, "./web/templates/"+filename)
	}

	webServer.templates, err = template.ParseFiles(templateFiles...)
	if err != nil {
		Logger.Fatal("parsing templates failed", zap.Error(err))
		panic("parsing templates failed")
	}
}

func (webServer *WebServer) Run() {
	go webServer.cacheUpdateJobs()
	go webServer.webSocketJobs()
	go webServer.listen()

	if Cfg.APIPort != 0 {
		go webServer.listenAPI()
	}
}

func (webServer *WebServer) webSocketHandler(w http.ResponseWriter, r *http.Request) {
	c, err := webServer.upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logger.Error("upgrading connection failed", zap.Error(err))
		return
	}

	blockInfo := webServer.getBlockInfo()
	if err := c.WriteJSON(&blockInfo); err != nil {
		return
	}

	poolStatsInfo := webServer.getPoolStatsInfo()
	if err := c.WriteJSON(&poolStatsInfo); err != nil {
		return
	}

	client := NewClient(c, webServer.finishedClients)
	webServer.newClients <- client
}

func GenMinerInfo(accountID uint64) *api.MinerInfo {
	miner := modelx.Cache.GetMiner(accountID)
	if miner == nil {
		return nil
	}

	poolCap := modelx.Cache.GetPoolCap()

	miner.Lock()
	cap := miner.CalculateEEPS()
	var historicalShare float64

	if poolCap != 0.0 {
		historicalShare = cap / poolCap
	}

	mi := &api.MinerInfo{
		ID:                    accountID,
		Address:               miner.Address,
		Name:                  miner.Name,
		Pending:               miner.Pending,
		EffectiveCapacity:     cap,
		HistoricalShare:       historicalShare,
		Deadline:              miner.CurrentDeadline(),
		LastActiveBlockHeight: miner.CurrentBlockHeight(),
		NConf:                 int32(len(miner.DeadlinesParams)),
		PayoutDetail:          miner.PayoutDetail}
	miner.Unlock()

	return mi
}

func (webServer *WebServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	indexInfo := &IndexInfo{
		Cfg:     &Cfg,
		NetDiff: webServer.netDiff.Load().(float64)}
	template := webServer.templates.Lookup("index.tmpl")
	template.ExecuteTemplate(w, "index", indexInfo)
}

func (webServer *WebServer) infoHandler(w http.ResponseWriter, r *http.Request) {
	indexInfo := &IndexInfo{
		Cfg:     &Cfg,
		NetDiff: webServer.netDiff.Load().(float64)}
	template := webServer.templates.Lookup("info.tmpl")
	template.ExecuteTemplate(w, "info", indexInfo)
}

func (webServer *WebServer) updateRecentlyWonBlocks() {
	webServer.wonBlocksMu.Lock()
	defer webServer.wonBlocksMu.Unlock()
	webServer.wonBlocks = webServer.modelx.GetRecentlyWonBlocks()
}

func (webServer *WebServer) updateMinerInfos() {
	webServer.minerInfosMu.Lock()

	var mis []*MinerInfo
	var poolCap float64
	var invDeadlineSum float64
	var minerCount int32
	currentBlock := modelx.Cache.CurrentBlock()
	modelx.Cache.MinerRange(func(k, v interface{}) bool {
		if v == nil {
			return true
		}
		miner := v.(*modelx.Miner)
		minerCount++

		capacity := miner.CalculateEEPS() * 1000.0
		miner.Lock()
		mi := &MinerInfo{
			ID:                    k.(uint64),
			Name:                  miner.Name,
			Address:               miner.Address,
			Pending:               burstmath.PlanckToBurst(miner.Pending),
			HistoricalShare:       0.0,
			NConf:                 len(miner.DeadlinesParams),
			Capacity:              capacity,
			Deadline:              miner.CurrentDeadline(),
			LastActiveBlockHeight: miner.CurrentBlockHeight()}
		miner.Unlock()
		mis = append(mis, mi)

		if mi.LastActiveBlockHeight == currentBlock.Height {
			invDeadlineSum += 1.0 / float64(mi.Deadline+1.0)
		}

		poolCap += capacity

		return true
	})

	if poolCap != 0.0 {
		for _, m := range mis {
			m.HistoricalShare = m.Capacity / poolCap * 100.0
			m.Capacity /= 1000.0
		}
		poolCap /= 1000.0
	}

	sort.Slice(mis, func(i, j int) bool { return mis[i].Deadline < mis[j].Deadline })

	webServer.minerInfos = mis
	modelx.Cache.StorePoolCap(poolCap)
	modelx.Cache.StoreMinerCount(minerCount)

	if invDeadlineSum == 0.0 {
		webServer.minerInfosMu.Unlock()
		return
	}

	// no one cares
	var shares []*Share
	var bestSharesSum float64
	for _, mi := range mis {
		if len(shares) == bestShares {
			break
		}
		if mi.LastActiveBlockHeight != currentBlock.Height {
			continue
		}

		var name string
		if mi.Name == "" {
			name = mi.Address
		} else {
			name = mi.Name
		}

		shareVal := (1.0 / float64(mi.Deadline+1.0)) / invDeadlineSum
		bestSharesSum += shareVal

		shares = append(shares, &Share{
			Val:  shareVal,
			Name: name})
	}

	otherShare := 1.0 - bestSharesSum
	if otherShare < 0.0 {
		otherShare = 0.0
	}
	shares = append(shares, &Share{
		Val:  otherShare,
		Name: "other"})

	webServer.minerInfosMu.Unlock()
	webServer.shareUpdates <- shares
}

func (webServer *WebServer) minersHandler(w http.ResponseWriter, r *http.Request) {
	webServer.minerInfosMu.RLock()
	defer webServer.minerInfosMu.RUnlock()

	template := webServer.templates.Lookup("minerTable.tmpl")
	template.ExecuteTemplate(w, "minerTable", webServer.minerInfos)
}

func (webServer *WebServer) wonBlocksHandler(w http.ResponseWriter, r *http.Request) {
	webServer.wonBlocksMu.RLock()
	defer webServer.wonBlocksMu.RUnlock()

	template := webServer.templates.Lookup("wonBlocks.tmpl")
	template.ExecuteTemplate(w, "wonBlocks", webServer.wonBlocks)
}

func (webServer *WebServer) listen() {
	http.HandleFunc("/ws", webServer.webSocketHandler)
	http.HandleFunc("/", webServer.indexHandler)
	http.HandleFunc("/miners", webServer.minersHandler)
	http.HandleFunc("/info", webServer.infoHandler)
	http.HandleFunc("/wonblocks", webServer.wonBlocksHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", Cfg.WebServerListenAddress, Cfg.WebServerPort), nil)
	if err != nil {
		Logger.Fatal("ListenAndServer failed", zap.Error(err))
	}
}

func (webServer *WebServer) listenAPI() {
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", Cfg.APIListenAddress, Cfg.APIPort))
	if err != nil {
		Logger.Fatal("failed to listen", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	api.RegisterApiServer(grpcServer, webServer)
	if err := grpcServer.Serve(lis); err != nil {
		Logger.Fatal("failed to server", zap.Error(err))
	}
}

func (webServer *WebServer) GetMinerInfo(ctx context.Context, req *api.MinerRequest) (*api.MinerInfo, error) {
	minerInfo := GenMinerInfo(req.ID)
	if minerInfo == nil {
		return nil, errors.New("unknown miner")
	}
	return minerInfo, nil
}

func (webServer *WebServer) GetPoolStatsInfo(ctx context.Context, req *api.Void) (*api.PoolStatsInfo, error) {
	poolStatsInfo := webServer.getPoolStatsInfo()
	return &poolStatsInfo, nil
}

func (webServer *WebServer) GetPoolConfigInfo(ctx context.Context, req *api.Void) (*api.PoolConfigInfo, error) {
	return &api.PoolConfigInfo{
		PoolFeeShare:    Cfg.PoolFeeShare,
		DeadlineLimit:   Cfg.DeadlineLimit,
		MinimumPayout:   Cfg.MinimumPayout,
		TxFee:           Cfg.PoolTxFee,
		WinnerShare:     Cfg.WinnerShare,
		TMin:            Cfg.TMin,
		NAVG:            int32(Cfg.NAVG),
		NMin:            int32(Cfg.NMin),
		SetNowFee:       Cfg.SetNowFee,
		SetDailyFee:     Cfg.SetDailyFee,
		SetWeeklyFee:    Cfg.SetWeeklyFee,
		SetMinPayoutFee: Cfg.SetMinPayoutFee,
		Version:         Cfg.Version,
		PoolPublicID:    Cfg.PoolPublicID}, nil
}

func (webServer *WebServer) GetBlockInfo(ctx context.Context, req *api.Void) (*api.BlockInfo, error) {
	blockInfo := webServer.getBlockInfo()
	return &blockInfo, nil
}

func (webServer *WebServer) getBlockInfo() api.BlockInfo {
	return webServer.blockInfo.Load().(api.BlockInfo)
}

func (webServer *WebServer) getPoolStatsInfo() api.PoolStatsInfo {
	return webServer.poolStatsInfo.Load().(api.PoolStatsInfo)
}

func (webServer *WebServer) sendToClients(t int, data []byte) {
	for client := range webServer.clients {
		client.QueueMsg(&WebSocketMsg{
			Type: t,
			Data: data})
	}
}

func (webServer *WebServer) checkForBlockUpdate() {
	blockInfo := webServer.getBlockInfo()
	newBlock := modelx.Cache.CurrentBlock()

	if newBlock.Height > blockInfo.Height {
		created, _ := newBlock.Created.MarshalText()
		blockInfo := api.BlockInfo{
			Height:              newBlock.Height,
			BaseTarget:          newBlock.BaseTarget,
			GenerationSignature: newBlock.GenerationSignature,
			Scoop:               newBlock.Scoop,
			Created:             string(created)}
		webServer.blockInfo.Store(blockInfo)
		webServer.blockUpdates <- &blockInfo
	}
}

func (webServer *WebServer) checkForNewBestSubmission() {
	newBestNonceSubmission := modelx.Cache.BestNonceSubmission()
	blockInfo := webServer.getBlockInfo()

	if blockInfo.Height == newBestNonceSubmission.Height &&
		(blockInfo.Deadline == 0 || blockInfo.Deadline > newBestNonceSubmission.Deadline) {
		blockInfo.Deadline = newBestNonceSubmission.Deadline
		blockInfo.MinerID = newBestNonceSubmission.MinerID
		if newBestNonceSubmission.Name != "" {
			blockInfo.Miner = newBestNonceSubmission.Name
		} else {
			blockInfo.Miner = newBestNonceSubmission.Address
		}
		webServer.blockInfo.Store(blockInfo)
		webServer.blockUpdates <- &blockInfo
	}
}

func (webServer *WebServer) updateNetDiff() {
	netDiff := webServer.modelx.GetAVGNetDiff(netDiffBlockRange)
	webServer.netDiff.Store(netDiff)
}

func (webServer *WebServer) updatePoolStats() {
	webServer.poolStatsInfo.Store(api.PoolStatsInfo{
		EffectivePoolCapacity: modelx.Cache.GetPoolCap(),
		NetDiff:               webServer.netDiff.Load().(float64),
		MinerCount:            modelx.Cache.GetMinerCount()})
}

func (webServer *WebServer) cacheUpdateJobs() {
	webServer.updateMinerInfos()
	webServer.updateRecentlyWonBlocks()
	webServer.updateNetDiff()
	webServer.updatePoolStats()

	minerInfoUpdateTicker := time.NewTicker(20 * time.Second)
	wonBlocksUpdateTicker := time.NewTicker(10 * time.Minute)
	netDiffUpdateTicker := time.NewTicker(30 * time.Minute)
	bestNonceSubmissionTicker := time.NewTicker(5 * time.Second)
	newBlockTicker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-minerInfoUpdateTicker.C:
			webServer.updateMinerInfos()
		case <-newBlockTicker.C:
			webServer.checkForBlockUpdate()
		case <-bestNonceSubmissionTicker.C:
			webServer.checkForNewBestSubmission()
		case <-wonBlocksUpdateTicker.C:
			webServer.updateRecentlyWonBlocks()
		case <-netDiffUpdateTicker.C:
			webServer.updateNetDiff()
		}
	}
}

func (webServer *WebServer) webSocketJobs() {
	pingTicker := time.NewTicker(pingPeriod)
	poolStatsTicker := time.NewTicker(time.Minute * 2)
	for {
		select {
		case client := <-webServer.newClients:
			webServer.clients[client] = true
		case client := <-webServer.finishedClients:
			delete(webServer.clients, client)
		case block := <-webServer.blockUpdates:
			data, err := json.Marshal(block)
			if err != nil {
				Logger.Error("failed to encode block to json", zap.Error(err))
			} else {
				webServer.sendToClients(websocket.TextMessage, data)
			}
		case shares := <-webServer.shareUpdates:
			data, err := json.Marshal(shares)
			if err != nil {
				Logger.Error("failed to encode shares to json", zap.Error(err))
			} else {
				webServer.sendToClients(websocket.TextMessage, data)
			}
		case <-pingTicker.C:
			webServer.sendToClients(websocket.PingMessage, []byte{})
		case <-poolStatsTicker.C:
			webServer.updatePoolStats()
			poolStatsInfo := webServer.getPoolStatsInfo()
			data, err := json.Marshal(&poolStatsInfo)
			if err != nil {
				Logger.Error("failed to encode pool stats to json", zap.Error(err))
			} else {
				webServer.sendToClients(websocket.TextMessage, data)
			}
		}
	}
}
