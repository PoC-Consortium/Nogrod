// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package modelx

import (
	"sync"
	"sync/atomic"
)

type cache struct {
	miners              sync.Map
	bestNonceSubmission atomic.Value
	currentBlock        atomic.Value
	poolCap             atomic.Value // gb
	minerCount          int32

	rewardRecipient   map[uint64]bool
	rewardRecipientMu sync.RWMutex
}

var Cache *cache

func InitCache() {
	c := cache{}
	c.StoreBestNonceSubmission(NonceSubmission{})
	c.StoreCurrentBlock(Block{})
	c.StorePoolCap(0.0)
	c.rewardRecipient = make(map[uint64]bool)
	Cache = &c
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

func (c *cache) StoreCurrentBlock(b Block) {
	c.currentBlock.Store(b)
}

func (c *cache) CurrentBlock() Block {
	return c.currentBlock.Load().(Block)
}

func (c *cache) StoreMiner(m *Miner) *Miner {
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
