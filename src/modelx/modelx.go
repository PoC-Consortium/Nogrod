// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package modelx

import (
	. "config"
	"container/list"
	"database/sql"
	"errors"
	"fmt"
	. "logger"
	"rsencoding"
	"strconv"
	"sync"
	"time"
	"util"
	"wallet"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"go.uber.org/zap"
)

type Account struct {
	ID      uint64
	Address string
	Name    string
	Pending int64
}

type Miner struct {
	ID uint64

	Address string
	Name    string `db:"name"`
	Pending int64

	CurrentDeadlineParams *DeadlineParams

	DeadlinesParams     *list.List
	WeightedDeadlineSum float64

	PayoutDetail string `db:"account.payout_detail"`

	sync.Mutex
}

type DeadlineParams struct {
	Deadline   uint64
	BaseTarget uint64
	Height     uint64
}

type Block struct {
	Height                   uint64
	BaseTarget               uint64 `db:"base_target"`
	Scoop                    uint32
	GenerationSignature      string `db:"generation_signature"`
	GenerationSignatureBytes []byte
	WinnerVerified           bool          `db:"winner_verified"`
	WinnerID                 sql.NullInt64 `db:"winner_id"`
	Reward                   sql.NullInt64
	BestNonceSubmissionID    sql.NullInt64 `db:"best_nonce_submission_id"`
	Created                  time.Time
	GenerationTime           int32 `db:"generation_time"`
}

type Transaction struct {
	ID          uint64
	Amount      int64
	RecipientID uint64 `db:"recipient_id"`
	Created     time.Time
}

type Modelx struct {
	db       *sqlx.DB
	walletDB *sqlx.DB
	wallet   wallet.Wallet

	newBlockMu sync.RWMutex

	slowBlockHeights *list.List
}

type NonceSubmission struct {
	Nonce    uint64
	Deadline uint64
	MinerID  uint64 `db:"miner.id"`
	Height   uint64 `db:"block.height"`
	Name     string `db:"account.name"`
	Address  string `db:"account.address"`
}

type WonBlock struct {
	WinnerName    string  `db:"account.name"`
	WinnerAddress string  `db:"account.address"`
	WinnerID      uint64  `db:"account.id"`
	Deadline      uint64  `db:"nonce_submission.deadline"`
	BaseTarget    uint64  `db:"base_target"`
	Height        uint64  `db:"height"`
	Reward        float64 `db:"reward"`
}

func NewModelX(wallet wallet.Wallet) *Modelx {
	db, err := sqlx.Connect("mysql", Cfg.DB.User+":"+Cfg.DB.Password+"@/"+Cfg.DB.Name+
		"?charset=utf8&parseTime=True&loc=Local")

	if err != nil {
		Logger.Fatal("failed to connect to database", zap.Error(err))
	}

	var walletDB *sqlx.DB
	if Cfg.WalletDB.Name != "" {
		walletDB, err = sqlx.Connect("mysql", Cfg.WalletDB.User+":"+Cfg.WalletDB.Password+"@/"+
			Cfg.WalletDB.Name+"?charset=utf8&parseTime=True&loc=Local")
		if err != nil {
			Logger.Fatal("failed to connect to database", zap.Error(err))
		}
	}

	modelx := Modelx{
		db:               db,
		walletDB:         walletDB,
		wallet:           wallet,
		slowBlockHeights: list.New()}

	if Cfg.FeeAccountID != 0 {
		modelx.createFeeAccount()
	}

	loaded := modelx.loadCurrentBlock()
	if loaded {
		modelx.cacheMiners()
	} else {
		miningInfo, err := modelx.wallet.GetMiningInfo()
		if err != nil {
			Logger.Fatal("getting inital mining info failed", zap.Error(err))
		}

		block, err := modelx.NewBlock(miningInfo.BaseTarget, miningInfo.GenerationSignature,
			miningInfo.Height)
		if err != nil {
			Logger.Fatal("creating first block failed", zap.Error(err))
		}
		Cache.StoreCurrentBlock(block)
	}

	return &modelx
}

func (modelx *Modelx) loadCurrentBlock() bool {
	var currentBlock Block
	err := modelx.db.Get(&currentBlock, "SELECT * FROM block ORDER BY height desc LIMIT 1")
	if err != nil {
		Logger.Error("getting current block from db failed", zap.Error(err))
		return false
	}

	genSigBytes, err := util.DecodeGeneratorSignature(currentBlock.GenerationSignature)
	if err != nil {
		Logger.Fatal("deconding generationSignature to byte array failed", zap.Error(err))
	}
	currentBlock.GenerationSignatureBytes = genSigBytes

	Cache.StoreCurrentBlock(currentBlock)
	return true
}

func (modelx *Modelx) cacheMiners() {
	type submissionData struct {
		Miner
		Deadline   uint64 `db:"deadline"`
		Height     uint64 `db:"height"`
		BaseTarget uint64 `db:"block.base_target"`
	}

	var slowBlockHeights []uint64
	sql := "SELECT height FROM block WHERE generation_time >= ? ORDER BY height DESC LIMIT ?"
	err := modelx.db.Select(&slowBlockHeights, sql, Cfg.TMin, Cfg.NAVG+1)
	if err != nil {
		Logger.Fatal("failed getting slowBlockHeights", zap.Error(err))
	}

	// nothing to cache
	if len(slowBlockHeights) == 0 {
		return
	}

	for _, h := range slowBlockHeights {
		modelx.slowBlockHeights.PushFront(h)
	}

	query, args, err := sqlx.In(`SELECT
                  account.id        "id",
                  account.address   "address",
                  COALESCE(account.name, '') "name",
                  account.pending   "pending",
                  deadline          "deadline",
                  block_height      "height",
                  block.base_target "block.base_target",
                  COALESCE(account.min_payout_value,
                           CONCAT(account.payout_interval, "|", account.next_payout_date),
                           "") "account.payout_detail"
                FROM nonce_submission
                  JOIN account ON nonce_submission.miner_id = account.id
                  JOIN block ON block.height = nonce_submission.block_height
                WHERE block.height IN (?)
                ORDER BY block_height ASC`, slowBlockHeights)

	if err != nil {
		Logger.Fatal("failed to create IN query", zap.Error(err))
	}
	query = modelx.db.Rebind(query)

	var sds []submissionData

	// because of uints we actually should check if currentHeight is smaller
	// than Cfg.NAVG, but we are at a sufficient block height already...
	if err := modelx.db.Select(&sds, query, args...); err != nil {
		Logger.Fatal("failed getting active miners from db", zap.Error(err))
	}

	currentHeight := slowBlockHeights[0]

	heights := make(map[uint64]bool)
	for i := range sds {
		heights[sds[i].Height] = true
		miner := Cache.GetMiner(sds[i].ID)
		if miner == nil {
			miner = &sds[i].Miner
			miner.DeadlinesParams = list.New()
			Cache.StoreMiner(miner)
		}

		dp := &DeadlineParams{
			Height:     sds[i].Height,
			BaseTarget: sds[i].BaseTarget,
			Deadline:   sds[i].Deadline}
		if sds[i].Height == currentHeight {
			miner.CurrentDeadlineParams = dp
		} else {
			miner.DeadlinesParams.PushBack(dp)
			miner.WeightedDeadlineSum += util.WeightDeadline(dp.Deadline, dp.BaseTarget)
		}
	}
}

func (modelx *Modelx) createFeeAccount() {
	modelx.db.MustExec(
		"INSERT IGNORE INTO account (id, address) VALUES (?, ?)",
		Cfg.FeeAccountID, rsencoding.Encode(Cfg.FeeAccountID))
}

func (modelx *Modelx) CleanDB() {
	Logger.Info("starting to cleanup db")

	currentBlock := Cache.CurrentBlock()
	modelx.db.MustExec("DELETE FROM block WHERE height < ?", currentBlock.Height-3000)
	modelx.db.MustExec(`DELETE FROM miner WHERE id NOT IN
                              (SELECT DISTINCT miner_id FROM nonce_submission)`)

	tx, err := modelx.db.Begin()
	if err != nil {
		Logger.Error("begining cleanup transaction failed", zap.Error(err))
		return
	}

	transferPendngSQL := `UPDATE account SET pending =
                               (SELECT * FROM ( SELECT SUM(pending) FROM account WHERE id NOT IN
                                 (SELECT id FROM miner) ) bold ) WHERE id = ?`
	_, err = tx.Exec(transferPendngSQL, Cfg.FeeAccountID)
	if err != nil {
		Logger.Error("transfering pending to fee account failed", zap.Error(err))
		tx.Rollback()
		return
	}

	deleteOldAccountsSQL := "DELETE FROM account WHERE id != ? AND id NOT IN (SELECT id FROM miner)"
	_, err = tx.Exec(deleteOldAccountsSQL, Cfg.FeeAccountID)
	if err != nil {
		Logger.Error("deleteing old accounts failed", zap.Error(err))
		tx.Rollback()
		return
	}

	tx.Commit()
}

func (modelx *Modelx) RereadMinerNames() {
	Cache.MinerRange(func(k, v interface{}) bool {
		minerID := k.(uint64)

		accountInfo, err := modelx.wallet.GetAccountInfo(minerID)
		if err != nil {
			return true
		}

		// only hold lock as long as necessary
		miner := v.(*Miner)
		miner.Lock()
		if miner.Name == accountInfo.Name {
			miner.Unlock()
			return true
		}
		miner.Name = accountInfo.Name
		miner.Unlock()

		_, err = modelx.db.Exec("UPDATE account SET name = ? WHERE id = ?", accountInfo.Name, miner.ID)
		if err != nil {
			Logger.Error("updating miner name failed", zap.Error(err))
		}

		return true
	})
}

func (modelx *Modelx) getMinerFromDB(accountID uint64) *Miner {
	sql := `SELECT
                  id,
	          address,
                  COALESCE(name, '') "name",
	          pending
	        FROM account WHERE id = ?`

	miner := Miner{}
	err := modelx.db.Get(&miner, sql, accountID)
	if err != nil {
		return nil
	}

	miner.DeadlinesParams = list.New()

	return &miner
}

func (modelx *Modelx) GetBestNonceSubmissionOnBlock(height uint64) (*NonceSubmission, error) {
	var ns NonceSubmission
	sql := `SELECT
                  nonce_submission.deadline,
                  nonce_submission.nonce,
                  miner.id "miner.id",
                  COALESCE(account.name, '') "account.name",
                  account.address "account.address",
                  block.height "block.height"
                FROM block
                  JOIN nonce_submission ON block.best_nonce_submission_id = nonce_submission.id
                  JOIN miner ON miner.id = nonce_submission.miner_id
                  JOIN account ON account.id = nonce_submission.miner_id
                WHERE block_height = ?`

	err := modelx.db.Get(&ns, sql, height)
	if err != nil {
		return nil, err
	}

	return &ns, nil
}

func (modelx *Modelx) NewBlock(baseTarget uint64, genSig string, height uint64) (Block, error) {
	modelx.newBlockMu.Lock()
	defer modelx.newBlockMu.Unlock()

	var newBlock Block

	genSigBytes, err := util.DecodeGeneratorSignature(genSig)
	if err != nil {
		return newBlock, err
	}

	wasSlowBlock := true
	oldBlock := Cache.CurrentBlock()
	if oldBlock.Height != 0 {
		generationTime, err := modelx.getGenerationTime(oldBlock.Height)
		if err != nil {
			Logger.Error("failed to get generationTime", zap.Error(err))
			return newBlock, err
		}

		modelx.db.MustExec("UPDATE block SET generation_time = ? WHERE height = ?",
			generationTime, oldBlock.Height)

		wasSlowBlock = generationTime >= Cfg.TMin
		if wasSlowBlock {
			modelx.slowBlockHeights.PushBack(oldBlock.Height)
			for modelx.slowBlockHeights.Len() > Cfg.NAVG {
				modelx.slowBlockHeights.Remove(modelx.slowBlockHeights.Front())
			}
		}
	}

	block := Block{
		Height:                   height,
		BaseTarget:               baseTarget,
		Scoop:                    util.CalculateScoop(height, genSigBytes),
		GenerationSignature:      genSig,
		GenerationSignatureBytes: genSigBytes,
		Created:                  time.Now()}

	sql := `INSERT INTO block (height, base_target, scoop, generation_signature, created) VALUES (?, ?, ?, ?, ?)`
	modelx.db.MustExec(sql, block.Height, block.BaseTarget, block.Scoop, block.GenerationSignature, block.Created)

	Cache.StoreCurrentBlock(block)

	Cache.MinerRange(func(key, value interface{}) bool {
		miner := value.(*Miner)
		if miner.CurrentBlockHeight() < block.Height-Cfg.InactiveAfterXBlocks {
			Cache.DeleteMiner(key.(uint64))
		} else if modelx.slowBlockHeights.Len() > 0 {
			miner.removeOldDeadlineParams(modelx.slowBlockHeights.Front().Value.(uint64))
		}

		if wasSlowBlock {
			miner.addDeadlineParams()
		}

		return true
	})

	if modelx.ConnectedToWalletDB() {
		modelx.CacheRewardRecipients()
	} else {
		Cache.StoreRewardRecipients(make(map[uint64]bool))
	}

	return block, nil
}

func (modelx *Modelx) createMiner(accountID uint64) (*Miner, error) {
	address := rsencoding.Encode(accountID)

	var name string
	accountInfo, err := modelx.wallet.GetAccountInfo(accountID)
	if err == nil {
		name = accountInfo.Name
	}

	tx, err := modelx.db.Begin()
	if err != nil {
		return nil, err
	}

	accountSQL := "INSERT IGNORE INTO account (id, address, name) VALUES (?, ?, ?)"
	_, err = tx.Exec(accountSQL, accountID, address, name)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	minerSQL := "INSERT IGNORE INTO miner (id) VALUES (?)"
	_, err = tx.Exec(minerSQL, accountID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Miner{
		ID:              accountID,
		Address:         address,
		Name:            name,
		Pending:         0,
		DeadlinesParams: list.New()}, nil
}

func (modelx *Modelx) FirstOrCreateMiner(accountID uint64) *Miner {
	cachedMiner := Cache.GetMiner(accountID)
	if cachedMiner != nil {
		return cachedMiner
	}

	var miner *Miner
	var err error
	miner = modelx.getMinerFromDB(accountID)
	if miner == nil {
		miner, err = modelx.createMiner(accountID)
		if err != nil {
			Logger.Error("creating miner failed", zap.Error(err))
			return nil
		}
	}

	miner.DeadlinesParams = list.New()

	// if already exists at this point we rather use the existing
	miner = Cache.StoreMiner(miner)

	return miner
}

func (miner *Miner) addDeadlineParams() {
	if miner.CurrentDeadlineParams == nil {
		return
	}

	// is this a new submission
	back := miner.DeadlinesParams.Back()
	if back == nil || back.Value.(*DeadlineParams).Height < miner.CurrentBlockHeight() {
		dp := *miner.CurrentDeadlineParams
		miner.DeadlinesParams.PushBack(&dp)
		miner.WeightedDeadlineSum += util.WeightDeadline(dp.Deadline, dp.BaseTarget)
	}
}

func (miner *Miner) CurrentBlockHeight() uint64 {
	if miner.CurrentDeadlineParams != nil {
		return miner.CurrentDeadlineParams.Height
	} else if e := miner.DeadlinesParams.Back(); e != nil {
		return e.Value.(*DeadlineParams).Height
	}
	return 0
}

func (miner *Miner) CurrentDeadline() uint64 {
	if miner.CurrentDeadlineParams == nil {
		return ^uint64(0)
	}
	return miner.CurrentDeadlineParams.Deadline
}

func (miner *Miner) removeOldDeadlineParams(heightLimit uint64) {
	for e := miner.DeadlinesParams.Front(); e != nil; e = e.Next() {
		d := e.Value.(*DeadlineParams)
		// because of uints we actually should check if currentHeight is smaller
		// than Cfg.NAVG, but we are at a sufficient block height already...
		if d.Height < heightLimit {
			miner.WeightedDeadlineSum -= util.WeightDeadline(d.Deadline, d.BaseTarget)
			miner.DeadlinesParams.Remove(e)
		} else {
			return
		}
	}
}

func (miner *Miner) CalculateEEPS() float64 {
	return util.EEPS(miner.DeadlinesParams.Len(), miner.WeightedDeadlineSum)
}

func (modelx *Modelx) createNonceSubmission(miner *Miner, block *Block, deadline, nonce uint64) error {
	sql := "INSERT INTO nonce_submission (miner_id, block_height, deadline, nonce) VALUES (?, ?, ?, ?)"
	modelx.db.MustExec(sql, miner.ID, block.Height, deadline, nonce)

	miner.CurrentDeadlineParams = &DeadlineParams{
		Height:     block.Height,
		BaseTarget: block.BaseTarget,
		Deadline:   deadline}

	return nil
}

func (modelx *Modelx) updateNonceSubmision(miner *Miner, block *Block, deadline, nonce uint64) error {
	sql := "UPDATE nonce_submission SET deadline = ?, nonce = ? WHERE miner_id = ? AND block_height = ?"
	_, err := modelx.db.Exec(sql, deadline, nonce, miner.ID, block.Height)
	if err != nil {
		return err
	}

	miner.CurrentDeadlineParams.Deadline = deadline

	return nil
}

func (modelx *Modelx) UpdateOrCreateNonceSubmission(miner *Miner, block *Block, deadline, nonce uint64) error {
	modelx.newBlockMu.RLock()
	defer modelx.newBlockMu.RUnlock()

	currentBlock := Cache.CurrentBlock()
	if currentBlock.Height != block.Height {
		return errors.New("submitted on old block")
	}

	if miner.CurrentBlockHeight() == block.Height {
		// Check if there was a better deadline submitted by this miner during this round
		if miner.CurrentDeadline() < deadline {
			return nil
		}
		return modelx.updateNonceSubmision(miner, block, deadline, nonce)
	}

	return modelx.createNonceSubmission(miner, block, deadline, nonce)
}

func (modelx *Modelx) UpdateBestSubmission(minerID, height uint64) {
	sql := `UPDATE block SET best_nonce_submission_id =
                  (SELECT id FROM nonce_submission WHERE block_height = block.height AND miner_id = ?)
                  WHERE height = ? `
	modelx.db.MustExec(sql, minerID, height)
}

func (modelx *Modelx) RewardBlocks() {
	currentBlock := Cache.CurrentBlock()

	type HeightCreatedNonce struct {
		Height  uint64
		Created time.Time
		Nonce   uint64 `db:"nonce_submission.nonce"`
	}

	modelx.db.MustExec(`UPDATE block SET winner_verified = 1
                           WHERE best_nonce_submission_id IS NULL and winner_verified = 0`)

	var hcns []HeightCreatedNonce
	sql := `SELECT
                  height,
                  created,
                  nonce_submission.nonce "nonce_submission.nonce"
                FROM block
                  JOIN nonce_submission ON nonce_submission.id = block.best_nonce_submission_id
                WHERE height <= ? AND winner_verified = 0 AND NOT block.best_nonce_submission_id IS NULL
                  ORDER BY height ASC`

	err := modelx.db.Select(&hcns, sql, currentBlock.Height-Cfg.BlockHeightPayoutDelay)
	if err != nil {
		Logger.Error("failed to fetch blocks without winner verified", zap.Error(err))
		return
	}

	// set dynamic min payout
	// for this we need to check transactions
	// since the block that has not been checked
	if len(hcns) > 0 {
		earliestCreated := hcns[0].Created
		earliestHeight := hcns[0].Height
		msgOf, err := modelx.wallet.GetIncomingMsgsSince(
			earliestCreated.Add(-time.Second*30), earliestHeight)
		if err != nil {
			Logger.Error("failed to get messages", zap.Error(err))
		} else {
			modelx.SetMinPayout(msgOf)
		}
	}

	for _, hcn := range hcns {
		wonBlock, blockInfo, err := modelx.wallet.WonBlock(hcn.Height, hcn.Nonce)
		if err != nil {
			Logger.Error("failed to determine if block was won", zap.Error(err))
			continue
		}
		if wonBlock {
			modelx.rewardBlock(blockInfo)
		} else {
			modelx.db.MustExec("UPDATE block SET winner_verified = 1 WHERE height = ?",
				hcn.Height)
		}
	}
}

func (modelx *Modelx) GetEEPSsOnBlock(height uint64, writeToDb bool) (map[uint64]float64, float64, error) {
	type EEPSArgs struct {
		MinerID             uint64  `db:"miner_id"`
		WeightedDeadlineSum float64 `db:"weighted_deadline_sum"`
		NConf               int     `db:"confirmed_deadlines"`
	}

	eepsSQL := `SELECT
                  miner_id "miner_id",
                  CAST(SUM(deadline * block.base_target) AS DOUBLE) "weighted_deadline_sum",
                  COUNT(deadline) "confirmed_deadlines"
                FROM nonce_submission
                  JOIN (SELECT base_target, height
                          FROM block WHERE height <= ? AND generation_time >= ? ORDER BY height DESC LIMIT ?)
                          AS block ON block.height = nonce_submission.block_height
                GROUP BY miner_id;`

	var eepsArgs []EEPSArgs
	err := modelx.db.Select(&eepsArgs, eepsSQL, height, Cfg.TMin, Cfg.NAVG)
	if err != nil {
		return nil, 0.0, err
	}

	eepsOf := make(map[uint64]float64)
	var eepsSum float64
	for _, args := range eepsArgs {
		eeps := util.EEPS(args.NConf, args.WeightedDeadlineSum)
		if writeToDb {
			modelx.db.MustExec("UPDATE miner SET capacity = ? WHERE id = ?",
				int32(eeps*1000.0), args.MinerID)
		}
		eepsSum += eeps
		eepsOf[args.MinerID] = eeps
	}

	return eepsOf, eepsSum, nil
}

func (modelx *Modelx) GetSharesOnBlock(height uint64) (map[uint64]float64, error) {
	eepsOf, eepsSum, err := modelx.GetEEPSsOnBlock(height, true)

	if err != nil {
		return nil, err
	}

	shareOf := make(map[uint64]float64)
	if eepsSum != 0.0 {
		for i := range eepsOf {
			shareOf[i] = eepsOf[i] / eepsSum
		}
	}

	return shareOf, nil
}

func (modelx *Modelx) rewardBlock(blockInfo *wallet.BlockInfo) {
	shareInPlanckOf := make(map[uint64]int64)
	totalReward := blockInfo.BlockReward*100000000 + blockInfo.TotalFeeNQT
	reward := totalReward

	var poolFee int64
	if Cfg.FeeAccountID != 0 {
		poolFee = util.Round(float64(reward) * Cfg.PoolFeeShare)
		shareInPlanckOf[Cfg.FeeAccountID] = poolFee
		reward -= poolFee
	}

	winnerReward := util.Round(float64(reward) * Cfg.WinnerShare)
	reward -= winnerReward

	shareOf, err := modelx.GetSharesOnBlock(blockInfo.Height)
	if err != nil {
		Logger.Error("failed to calculate shares", zap.Error(err))
	}

	for accountID, share := range shareOf {
		shareInPlanck := util.Round(share * float64(reward))
		if accountID == blockInfo.GeneratorID {
			shareInPlanck += winnerReward
		}
		shareInPlanckOf[accountID] = shareInPlanck
	}

	// write into db
	tx, err := modelx.db.Begin()
	if err != nil {
		Logger.Error("beginning rewardBlock transaction failed", zap.Error(err))
		return
	}

	stmt, err := tx.Prepare("UPDATE account SET pending = pending + ? WHERE id = ?")
	if err != nil {
		Logger.Error("failed to prepare update pending stmt", zap.Error(err))
		tx.Rollback()
		return
	}

	for accountID, shareInPlanck := range shareInPlanckOf {
		_, err := stmt.Exec(shareInPlanck, accountID)
		if err != nil {
			Logger.Error("increasing pending failed", zap.Error(err))
			tx.Rollback()
			return
		}
	}

	sql := "UPDATE block SET winner_verified = 1, reward = ?, winner_id = ? WHERE height = ?"
	if _, err := tx.Exec(sql, totalReward, blockInfo.GeneratorID, blockInfo.Height); err != nil {
		Logger.Error("udpate won block failed", zap.Error(err))
		tx.Rollback()
		return
	}

	if err := tx.Commit(); err != nil {
		Logger.Error("rewardBlock transaction failed", zap.Error(err))
		return
	}

	// udpate cache, separate loop, because we don't want to lock inside the transaction
	for accountID, shareInPlanck := range shareInPlanckOf {
		if cachedMiner := Cache.GetMiner(accountID); cachedMiner != nil {
			cachedMiner.Lock()
			cachedMiner.Pending += shareInPlanck
			cachedMiner.Unlock()
		}
	}
}

func (modelx *Modelx) Payout() {
	type PendingInfo struct {
		ID             uint64
		PayoutInterval sql.NullString `db:"payout_interval"`
		Pending        int64
	}

	var pendingInfos []PendingInfo
	sql := `SELECT
                  id,
                  pending,
                  payout_interval "payout_interval"
                FROM account WHERE
                  (min_payout_value IS NOT NULL AND pending >= min_payout_value + ?) OR
                  (next_payout_date IS NOT NULL AND next_payout_date <= NOW() AND pending >= ?) OR
                  (min_payout_value IS NULL AND next_payout_date IS NULL AND pending >= ?)`
	err := modelx.db.Select(&pendingInfos, sql,
		Cfg.TxFee,
		Cfg.TxFee,
		Cfg.MinimumPayout+Cfg.TxFee)
	if err != nil {
		Logger.Error("fetching miners for payout failed", zap.Error(err))
		return
	}

	for _, pendingInfo := range pendingInfos {
		tx, err := modelx.db.Begin()
		if err != nil {
			Logger.Error("beginning payBlock transaction failed", zap.Error(err))
			continue
		}

		sql := "UPDATE account SET pending = pending - ? WHERE id = ?"
		if _, err = tx.Exec(sql, pendingInfo.Pending, pendingInfo.ID); err != nil {
			Logger.Error("decreasing pending failed", zap.Error(err))
			tx.Rollback()
			continue
		}

		if pendingInfo.PayoutInterval.Valid {
			sql := "UPDATE account SET next_payout_date = ? WHERE id = ?"
			var err error
			switch pendingInfo.PayoutInterval.String {
			case "weekly":
				_, err = tx.Exec(sql, time.Now().AddDate(0, 0, 7), pendingInfo.ID)
			case "daily":
				_, err = tx.Exec(sql, time.Now().AddDate(0, 0, 1), pendingInfo.ID)
			case "now":
				_, err = tx.Exec(sql, nil, pendingInfo.ID)
			}

			if err != nil {
				Logger.Error("failed to update next_payout_date", zap.Error(err))
				tx.Rollback()
				continue
			}
		}

		txID, err := modelx.wallet.SendPayment(pendingInfo.ID, pendingInfo.Pending-Cfg.TxFee)
		if err != nil {
			tx.Rollback()
			continue
		}

		sql = "INSERT INTO transaction (id, amount, recipient_id) VALUES (?, ?, ?)"
		if _, err = tx.Exec(sql, txID, pendingInfo.Pending-Cfg.TxFee, pendingInfo.ID); err != nil {
			Logger.Error("creating transaction failed", zap.Error(err))
			tx.Rollback()
			continue
		}

		if err := tx.Commit(); err != nil {
			Logger.Error("payout transaction failed", zap.Error(err))
		}

		if cachedMiner := Cache.GetMiner(pendingInfo.ID); cachedMiner != nil {
			cachedMiner.Lock()
			cachedMiner.Pending -= pendingInfo.Pending
			cachedMiner.Unlock()
		}
	}
}

func (modelx *Modelx) GetRecentlyWonBlocks() []WonBlock {
	sql := `SELECT
                  COALESCE(account.name, '') "account.name",
                  account.id       "account.id",
                  account.address  "account.address",
                  nonce_submission.deadline "nonce_submission.deadline",
                  base_target "base_target",
                  height      "height",
                  reward / 100000000.0      "reward"
                FROM block
                  JOIN account ON block.winner_id = account.id
                  JOIN nonce_submission ON block.best_nonce_submission_id = nonce_submission.id
                WHERE NOT winner_id IS NULL ORDER BY height DESC LIMIT 100`

	var wonBlocks []WonBlock
	if err := modelx.db.Select(&wonBlocks, sql); err != nil {
		Logger.Error("fetching recentlyWonBlocks from db failed", zap.Error(err))
	}

	return wonBlocks
}

func (modelx *Modelx) ConnectedToWalletDB() bool {
	return modelx.walletDB != nil
}

func (modelx *Modelx) IsPoolRewardRecipient(accountID uint64) (bool, error) {
	var isCorrect bool

	// try a cache lookup
	isCorrect, cached := Cache.IsRewardRecipient(accountID)
	if cached {
		return isCorrect, nil
	}

	// try to find in wallet db
	if modelx.ConnectedToWalletDB() {
		sql := `SELECT 1 FROM reward_recip_assign
                          WHERE account_id = CAST(? AS SIGNED) AND recip_id = CAST(? AS SIGNED) AND latest = 1 LIMIT 1`

		// ignore error no rows in resultset
		modelx.walletDB.Get(&isCorrect, sql, accountID, Cfg.PoolPublicID)
		Cache.StoreRewardRecipient(accountID, isCorrect)
		return isCorrect, nil
	}

	// ask wallet
	isCorrect, err := modelx.wallet.IsPoolRewardRecipient(accountID)
	if err == nil {
		Cache.StoreRewardRecipient(accountID, isCorrect)
	}
	return isCorrect, err
}

func (modelx *Modelx) getGenerationTime(height uint64) (int32, error) {
	// TODO: timestamp of block isn't available fast enough
	// if modelx.ConnectedToWalletDB() {
	// 	var timestamps []int32
	// 	sql := "SELECT timestamp FROM block WHERE height IN (?, ?) ORDER BY height DESC"
	// 	err := modelx.walletDB.Select(&timestamps, sql, height, height-1)
	// 	if err != nil {
	// 		return 0, err
	// 	}
	// 	return timestamps[0] - timestamps[1], nil
	// }
	return modelx.wallet.GetGenerationTime(height)
}

func (modelx *Modelx) CacheRewardRecipients() {
	var validAccoundIDs []uint64
	rewardRecipient := make(map[uint64]bool)
	defer Cache.StoreRewardRecipients(rewardRecipient)

	sql := `SELECT CAST(account_id AS UNSIGNED)
                  FROM reward_recip_assign
                  WHERE recip_id = CAST(? AS SIGNED) AND latest = 1`

	err := modelx.walletDB.Select(&validAccoundIDs, sql, Cfg.PoolPublicID)
	if err != nil {
		Logger.Error("failed caching reward recipients", zap.Error(err))
		return
	}

	for _, accoundID := range validAccoundIDs {
		rewardRecipient[accoundID] = true
	}
}

func (modelx *Modelx) GetAVGNetDiff(n uint) float64 {
	var netDiff float64
	err := modelx.db.Get(&netDiff,
		"SELECT 18325193796 / AVG(base_target) FROM block ORDER BY height DESC LIMIT ?", n)
	if err != nil {
		Logger.Error("failed to get netDiff", zap.Error(err))
		return 250000.0
	}
	return netDiff
}

func (modelx *Modelx) SetMinPayout(msgOf map[uint64]string) {
	pendingBigEnoughSQL := `SELECT 1 FROM account WHERE id = ? AND pending >= ?`
	setMinPayoutSQL := `UPDATE account SET
                  min_payout_value = ?,
                  payout_interval = ?,
                  next_payout_date = ?,
                  pending = pending - ?
                 WHERE id = ?`
	transferFeeSQL := `UPDATE account SET pending = pending + ? WHERE id = ?`
	for accountID, msg := range msgOf {
		var cost int64
		var nextPayoutDate *time.Time
		var payoutInterval *string
		var minPayoutValue *int64

		var oldMsg bool
		switch msg {
		case "weekly":
			tmpStr := "weekly"
			payoutInterval = &tmpStr
			tmpTime := time.Now().AddDate(0, 0, 7)
			nextPayoutDate = &tmpTime
			cost = Cfg.SetWeeklyFee

			modelx.db.QueryRow(`SELECT 1 FROM account WHERE id = ? AND payout_interval = "weekly"`,
				accountID).Scan(&oldMsg)
			if oldMsg {
				Logger.Info("processed msg second time", zap.Uint64("account id", accountID))
				continue
			}
		case "daily":
			tmpStr := "daily"
			payoutInterval = &tmpStr
			tmpTime := time.Now().AddDate(0, 0, 1)
			nextPayoutDate = &tmpTime
			cost = Cfg.SetDailyFee

			modelx.db.QueryRow(`SELECT 1 FROM account WHERE id = ? AND payout_interval = "daily"`,
				accountID).Scan(&oldMsg)
			if oldMsg {
				Logger.Info("processed msg second time", zap.Uint64("account id", accountID))
				continue
			}
		case "now":
			tmpStr := "now"
			payoutInterval = &tmpStr
			tmpTime := time.Now()
			nextPayoutDate = &tmpTime
			cost = Cfg.SetNowFee

			modelx.db.QueryRow(`SELECT 1 FROM account WHERE id = ? AND payout_interval = "now"`,
				accountID).Scan(&oldMsg)
			if oldMsg {
				Logger.Info("processed msg second time", zap.Uint64("account id", accountID))
				continue
			}
		default:
			minPayout, parseErr := strconv.ParseInt(msg, 10, 64)
			if parseErr != nil {
				minPayoutFloat, parseErr := strconv.ParseFloat(msg, 64)
				if parseErr != nil {
					Logger.Error("failed to parse minPayout", zap.Error(parseErr))
					continue
				}
				minPayout = int64(minPayoutFloat)
			}
			if minPayout != 0 {
				minPayoutValue = &minPayout
				modelx.db.QueryRow(`SELECT 1 FROM account WHERE id = ? AND min_payout_value = ?`,
					accountID, minPayout).Scan(&oldMsg)
				if oldMsg {
					Logger.Info("processed msg second time", zap.Uint64("account id", accountID))
					continue
				}

				cost = Cfg.SetMinPayoutFee
			}
		}

		tx, err := modelx.db.Begin()
		if err != nil {
			Logger.Error("failed to start setMinPayout transaction", zap.Error(err))
			continue
		}

		var enoughFunds bool
		err = tx.QueryRow(pendingBigEnoughSQL, accountID, cost).Scan(&enoughFunds)
		if err != nil {
			tx.Rollback()
			Logger.Error("pending not big enough of miner", zap.Error(err))
			continue
		}
		if !enoughFunds {
			tx.Rollback()
			Logger.Error("not enough funds to update dynamic payout",
				zap.Uint64("accountID", accountID),
				zap.Int64("cost", cost))
			continue
		}

		_, err = tx.Exec(setMinPayoutSQL, minPayoutValue, payoutInterval, nextPayoutDate, cost, accountID)
		if err != nil {
			tx.Rollback()
			Logger.Error("failed to update minPayout", zap.Error(err))
			continue
		}

		_, err = tx.Exec(transferFeeSQL, cost, Cfg.FeeAccountID)
		if err != nil {
			tx.Rollback()
			Logger.Error("failed to transfer fee to fee account", zap.Error(err))
			continue
		}

		err = tx.Commit()
		if err != nil {
			Logger.Error("set minPayout transaction failed", zap.Error(err))
		}

		if miner := Cache.GetMiner(accountID); miner != nil {
			miner.Lock()
			if minPayoutValue != nil {
				miner.PayoutDetail = fmt.Sprint(*minPayoutValue)
			} else if payoutInterval != nil {
				if *payoutInterval == "weekly" {
					miner.PayoutDetail = "weekly|" + nextPayoutDate.String()
				} else if *payoutInterval == "daily" {
					miner.PayoutDetail = "daily|" + nextPayoutDate.String()
				}
			}
			Logger.Info("set payout", zap.Uint64("account id", accountID), zap.Int64("cost", cost))
			miner.Pending -= cost
			miner.Unlock()
		}
	}
}
