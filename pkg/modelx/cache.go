// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package modelx

import (
	"container/list"
	"encoding/json"
	"math"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/PoC-Consortium/Nogrod/pkg/config"
)

var Cache *cache

type cache struct {
	miners              sync.Map
	bestNonceSubmission atomic.Value
	currentBlock        atomic.Value
	poolCap             atomic.Value // gb
	minerCount          int32

	rewardRecipient   map[uint64]bool
	rewardRecipientMu sync.RWMutex

	slowBlocks *blocks
	fastBlocks *blocks

	alphas []float64

	miningInfoJSON atomic.Value
	roundInfo      atomic.Value
}

type blocks struct {
	heights *list.List
	index   map[uint64]*list.Element
	maxLen  int
	sync.RWMutex
}

type RoundInfo struct {
	Scoop               uint32
	BaseTarget          uint64
	Height              uint64
	GenSig              []byte
	RoundStart          time.Time
	GenerationSignature string
}

func InitCache() {
	c := cache{}
	c.StoreBestNonceSubmission(NonceSubmission{})
	c.StoreCurrentBlock(Block{})
	c.StorePoolCap(0.0)
	c.rewardRecipient = make(map[uint64]bool)
	c.computeAlphas(Cfg.NAVG, Cfg.NMin)
	c.slowBlocks = newBlocks(Cfg.NAVG)
	c.fastBlocks = newBlocks(Cfg.NAVG)
	Cache = &c
}

func newBlocks(maxLen int) *blocks {
	if maxLen <= 0 {
		panic("maxLen must be bigger 0")
	}
	return &blocks{
		maxLen:  maxLen,
		heights: list.New(),
		index:   make(map[uint64]*list.Element)}
}

func (blocks *blocks) add(height uint64) uint64 {
	// If the height is already cached we won't add id
	// If we insert a new element, we also have to take care of removing
	// old heights if cache is full
	blocks.Lock()
	if _, exists := blocks.index[height]; exists {
		blocks.Unlock()
		return 0
	}
	for e := blocks.heights.Back(); e != nil; e = e.Prev() {
		curHeight := e.Value.(uint64)
		if curHeight < height {
			blocks.index[height] = blocks.heights.InsertAfter(height, e)
			if blocks.maxLen < blocks.heights.Len() {
				oldHeight := blocks.heights.Remove(blocks.heights.Front()).(uint64)
				delete(blocks.index, oldHeight)
				blocks.Unlock()
				return oldHeight
			}
			blocks.Unlock()
			return 0
		}
	}
	if blocks.maxLen > blocks.heights.Len() {
		blocks.index[height] = blocks.heights.PushFront(height)
	}
	blocks.Unlock()
	return 0
}

func (blocks *blocks) exists(height uint64) bool {
	blocks.RLock()
	_, exists := blocks.index[height]
	blocks.RUnlock()
	return exists
}

func (c *cache) StorePoolCap(cap float64) {
	c.poolCap.Store(cap)
}

func (c *cache) GetPoolCap() float64 {
	return c.poolCap.Load().(float64)
}

func (c *cache) StoreMinerCount(i int32) {
	atomic.StoreInt32(&c.minerCount, i)
}

func (c *cache) GetMinerCount() int32 {
	return atomic.LoadInt32(&c.minerCount)
}

func (c *cache) StoreRoundInfo(b Block) {
	c.currentBlock.Store(b)
	c.roundInfo.Store(RoundInfo{
		Scoop:               b.Scoop,
		BaseTarget:          b.BaseTarget,
		Height:              b.Height,
		GenSig:              b.GenerationSignatureBytes,
		GenerationSignature: b.GenerationSignature,
		RoundStart:          b.Created})
}

func (c *cache) StoreMiningInfo(b *Block) {
	miningInfoBytes, _ := json.Marshal(map[string]interface{}{
		"baseTarget":          b.BaseTarget,
		"generationSignature": b.GenerationSignature,
		"height":              b.Height,
		"targetDeadline":      Cfg.DeadlineLimit})
	c.miningInfoJSON.Store(miningInfoBytes)
}

func (c *cache) StoreCurrentBlock(b Block) {
	c.StoreRoundInfo(b)
	c.StoreMiningInfo(&b)
}

func (c *cache) GetMiningInfoJSON() []byte {
	return c.miningInfoJSON.Load().([]byte)
}

func (c *cache) GetRoundInfo() RoundInfo {
	return c.roundInfo.Load().(RoundInfo)
}

func (c *cache) CurrentBlock() Block {
	return c.currentBlock.Load().(Block)
}

func (c *cache) LoadOrStoreMiner(m *Miner) *Miner {
	v, _ := c.miners.LoadOrStore(m.ID, m)
	return v.(*Miner)
}

func (c *cache) GetMiner(id uint64) *Miner {
	v, isCached := c.miners.Load(id)
	if isCached {
		return v.(*Miner)
	}
	return nil
}

func (c *cache) MinerRange(f func(id, m interface{}) bool) {
	c.miners.Range(f)
}

func (c *cache) DeleteMiner(id uint64) {
	c.miners.Delete(id)
}

func (c *cache) IsRewardRecipient(id uint64) (bool, bool) {
	c.rewardRecipientMu.RLock()
	defer c.rewardRecipientMu.RUnlock()

	isCorrect, ok := c.rewardRecipient[id]
	if !ok {
		return false, false
	}
	return isCorrect, true
}

func (c *cache) StoreRewardRecipient(id uint64, isCorrect bool) {
	c.rewardRecipientMu.Lock()
	defer c.rewardRecipientMu.Unlock()
	c.rewardRecipient[id] = isCorrect
}

func (c *cache) StoreRewardRecipients(rewardRecipient map[uint64]bool) {
	c.rewardRecipientMu.Lock()
	defer c.rewardRecipientMu.Unlock()
	c.rewardRecipient = rewardRecipient
}

func (c *cache) StoreBestNonceSubmission(bestNonceSubmission NonceSubmission) {
	c.bestNonceSubmission.Store(bestNonceSubmission)
}

func (c *cache) BestNonceSubmission() NonceSubmission {
	return c.bestNonceSubmission.Load().(NonceSubmission)
}

func (c *cache) alpha(nConf int) float64 {
	if nConf == 0 {
		return 0.0
	}
	if len(c.alphas) < nConf {
		return 1.0
	}
	return c.alphas[nConf-1]
}

func (c *cache) computeAlphas(nAvg int, nMin int) {
	c.alphas = make([]float64, nAvg)
	for i := 0; i < nAvg; i++ {
		if i < nMin-1 {
			c.alphas[i] = 0.0
		} else {
			nConf := float64(i + 1)
			c.alphas[i] = 1.0 - (float64(nAvg)-nConf)/nConf*math.Log(float64(nAvg)/(float64(nAvg)-nConf))
		}
	}
	c.alphas[nAvg-1] = 1.0
}

func (c *cache) WasSlowBlock(height uint64) (bool, bool) {
	if c.slowBlocks.exists(height) {
		return true, true
	}
	if c.fastBlocks.exists(height) {
		return false, true
	}
	return false, false
}

func (c *cache) AddSlowBlock(height uint64) uint64 {
	return c.slowBlocks.add(height)
}

func (c *cache) AddBlock(height uint64, generationTime int32) uint64 {
	if generationTime < Cfg.TMin {
		return c.fastBlocks.add(height)
	}
	return c.slowBlocks.add(height)
}
