// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package modelx

import (
	"database/sql"
	"errors"
	"log"
	"os/exec"
	"testing"
	"time"

	. "config"
	"mocks"
	"wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	user         = "root"
	sampleGenSig = "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9"
)

var modelx *Modelx
var walletHandlerMock mocks.WalletHandler

func init() {
	LoadConfig()

	cmd := "echo 'DROP DATABASE IF EXISTS testburstpool; CREATE DATABASE testburstpool;' | mysql -u" + user
	_, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}

	cmd = "mysql -u" + user + " testburstpool < testburstpool.sql"
	_, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}

	InitCache()
	modelx = NewModelX(&walletHandlerMock)
}

func TestNewModelX(t *testing.T) {
	if !assert.NotNil(t, modelx, "creating model failed") {
		log.Fatal("creating model failed")
	}

	dummyTime := time.Now()
	expBlock := Block{
		Height:         493747,
		BaseTarget:     48450,
		Scoop:          17,
		Created:        dummyTime,
		GenerationTime: 240,
		BestNonceSubmissionID: sql.NullInt64{
			Valid: true,
			Int64: 125652},
		GenerationSignature: "19350893398e50c6cf58b885ba378ac82b65cc9c24c6bd83b90d084b7f872160",
		GenerationSignatureBytes: []uint8{
			0x19, 0x35, 0x08, 0x93, 0x39, 0x8e, 0x50, 0xc6,
			0xcf, 0x58, 0xb8, 0x85, 0xba, 0x37, 0x8a, 0xc8,
			0x2b, 0x65, 0xcc, 0x9c, 0x24, 0xc6, 0xbd, 0x83,
			0xb9, 0x0d, 0x08, 0x4b, 0x7f, 0x87, 0x21, 0x60}}
	block := Cache.CurrentBlock()
	block.Created = dummyTime
	assert.Equal(t, expBlock, block)

	type minerTest struct {
		id              uint64
		address         string
		name            string
		pending         int64
		currentDeadline uint64
		currentHeight   uint64
		eeps            float64
	}
	minerTests := []minerTest{
		minerTest{
			id:              8686227335924170201,
			address:         "SHGT-RQ82-92A6-9FX4K",
			name:            "阔尔吉UP",
			currentDeadline: 65763,
			currentHeight:   493745,
			eeps:            787.8953572355927},
		minerTest{
			id:              14250239474703782444,
			address:         "HNKE-BNGZ-86QH-E7SJD",
			name:            "NOVA Mining",
			currentDeadline: 33429,
			currentHeight:   493744,
			eeps:            170.15274522825118,
			pending:         10000000000}}
	for _, minerTest := range minerTests {
		miner := Cache.GetMiner(minerTest.id)
		if !assert.NotNil(t, miner) {
			continue
		}
		assert.Equal(t, minerTest.id, miner.ID)
		assert.Equal(t, minerTest.address, miner.Address)
		assert.Equal(t, minerTest.name, miner.Name)
		assert.Equal(t, minerTest.pending, miner.Pending)
		assert.Equal(t, minerTest.currentDeadline, miner.CurrentDeadline())
		assert.Equal(t, minerTest.currentHeight, miner.CurrentBlockHeight())
		assert.Equal(t, minerTest.eeps, miner.CalculateEEPS())
		assert.Equal(t, minerTest.pending, miner.Pending)
	}

	fastBlockHeights := []uint64{493695, 493657}
	for _, h := range fastBlockHeights {
		slow, exists := Cache.WasSlowBlock(h)
		if assert.True(t, exists) {
			assert.False(t, slow, "", h)
		}
	}

	slowBlockHeights := []uint64{493746, 493694, 493652}
	for _, h := range slowBlockHeights {
		slow, exists := Cache.WasSlowBlock(h)
		if assert.True(t, exists) {
			assert.True(t, slow)
		}
	}
}

func TestGetSharesOnBlock(t *testing.T) {
	shares, err := modelx.GetSharesOnBlock(1)
	if assert.Nil(t, err) {
		assert.Equal(t, 0, len(shares), "got shares, but excpeted none")
	}

	shares, err = modelx.GetSharesOnBlock(493725)
	if assert.Nil(t, err) {
		assert.Equal(t, 35, len(shares), "wrong number of entries in shares")
		assert.Equal(t, 0.062089898438717844, shares[8686227335924170201])
		assert.Equal(t, 0.012949809323534489, shares[14250239474703782444])
	}
}

func TestGetRecentlyWonBlocks(t *testing.T) {
	blocks := modelx.GetRecentlyWonBlocks()
	if assert.Equal(t, 100, len(blocks), "amount of recently won blocks wrong") {
		b := blocks[0]
		assert.Equal(t, uint64(64), b.Deadline, "deadline wrong")
		assert.Equal(t, uint64(493697), b.Height, "height wrong")
		assert.Equal(t, 994.0, b.Reward, "reward wrong")
		assert.Equal(t, "VDHA-GAKV-HTTN-2MQS8", b.WinnerAddress, "winner address wrong")
		assert.Equal(t, uint64(243989817010793960), b.WinnerID, "winner id wrong")
		assert.Equal(t, "1000TT", b.WinnerName, "winner name wrong")
	}
}

func TestRewardBlocks(t *testing.T) {
	heightsToCheck := []uint64{492024, 491588, 493698, 492024, 493735, 493736, 493737}

	walletHandlerMock.On("GetIncomingMsgsSince", mock.Anything).Return(map[uint64]string{}, nil)

	modelx.db.MustExec("UPDATE block SET best_nonce_submission_id = 125547 WHERE height = 493735")
	modelx.db.MustExec("UPDATE block SET best_nonce_submission_id = 125641 WHERE height = 493745")

	modelx.db.MustExec("UPDATE block SET best_nonce_submission_id = NULL WHERE height = 493730")
	modelx.db.MustExec("DELETE FROM nonce_submission WHERE block_height = 493730")

	for _, h := range heightsToCheck {
		modelx.db.MustExec("UPDATE block SET winner_verified = 0 WHERE height = ?", h)
	}

	walletHandlerMock.On("WonBlock", uint64(492024), mock.Anything, mock.Anything).Return(false, nil, nil)
	walletHandlerMock.On("WonBlock", uint64(493735), mock.Anything, mock.Anything).Return(false, nil, nil)
	walletHandlerMock.On("WonBlock", uint64(493736), mock.Anything, mock.Anything).Return(false, nil, nil)
	walletHandlerMock.On("WonBlock", uint64(493737), mock.Anything, mock.Anything).Return(false, nil, nil)
	walletHandlerMock.On("WonBlock", uint64(492024), uint64(243989817010793960), uint64(626112363318)).Return(
		true, &wallet.BlockInfo{
			Height:      492024,
			GeneratorID: 243989817010793960,
			BlockReward: 99500000000,
			TotalFeeNQT: 500000000}, nil)
	walletHandlerMock.On("WonBlock", uint64(491588), uint64(243989817010793960), uint64(641317572514)).Return(
		true, &wallet.BlockInfo{
			Height:      491588,
			GeneratorID: 243989817010793960,
			BlockReward: 39500000000,
			TotalFeeNQT: 400000000}, nil)
	walletHandlerMock.On("WonBlock", uint64(493698), uint64(10687838508612871566), uint64(11076047877)).Return(
		true, &wallet.BlockInfo{
			Height:      493698,
			GeneratorID: 243989817010793960,
			BlockReward: 39500000000,
			TotalFeeNQT: 400000000}, nil)

	modelx.RewardBlocks()

	var winnerVerified bool
	err := modelx.db.Get(&winnerVerified, "SELECT winner_verified FROM block WHERE height = 493730")
	if assert.Nil(t, err) {
		assert.True(t, winnerVerified,
			"did not set winner verified to true, when there are not nonce submissions on this block")
	}

	var bestNonceSubmissionID uint64
	err = modelx.db.Get(&bestNonceSubmissionID,
		"SELECT best_nonce_submission_id FROM block WHERE height = 493735")
	if assert.Nil(t, err) {
		assert.Equal(t, uint64(125549), bestNonceSubmissionID, "didn't update best nonce submission id")
	}

	err = modelx.db.Get(&bestNonceSubmissionID,
		"SELECT best_nonce_submission_id FROM block WHERE height = 493745")
	if assert.Nil(t, err) {
		assert.Equal(t, uint64(125641), bestNonceSubmissionID, "udpated best nonce submission id")
	}

	type pendingTest struct {
		pending   int64
		accountID uint64
	}

	pendingTests := []pendingTest{
		pendingTest{
			accountID: 243989817010793960,
			pending:   4702545303989440960},
		pendingTest{
			accountID: 6418289488649374107,
			pending:   0},
		pendingTest{
			accountID: 3685541669762741899,
			pending:   0},
		pendingTest{
			accountID: 10687838508612871566,
			pending:   283464053812433856},
		pendingTest{
			accountID: 9447004673583704489,
			pending:   74140093634441372},
		pendingTest{
			accountID: 16724824580964856856,
			pending:   16931731334867458}}

	for _, test := range pendingTests {
		var pending int64
		err := modelx.db.Get(&pending, "SELECT pending FROM account WHERE id = ?", test.accountID)
		if assert.Nil(t, err) {
			assert.Equal(t, test.pending, pending, "updated pending correctly")

			// skip pool fee account (not in cache)
			if miner := Cache.GetMiner(test.accountID); miner != nil {
				assert.Equal(t, test.pending, miner.Pending, "updated pending correctly (cache)")
			}
		}
	}

	for _, h := range heightsToCheck {
		var winnerVerified bool
		err := modelx.db.Get(&winnerVerified, "SELECT winner_verified FROM block WHERE height = ?", h)
		if assert.Nil(t, err) {
			assert.True(t, winnerVerified, "didn't set winner verified flag", h)
		}
	}
}

func TestFirstOrCreateMiner(t *testing.T) {
	accountID := uint64(1337)
	walletHandlerMock.On("GetAccountInfo", accountID).Return(&wallet.AccountInfo{Name: "josef"}, nil)
	miner := modelx.FirstOrCreateMiner(accountID)
	if !assert.NotNil(t, miner, "failed to create miner") {
		return
	}

	assert.Equal(t, miner, Cache.GetMiner(accountID), "didn't cache miner")
	assert.Equal(t, miner, modelx.FirstOrCreateMiner(accountID), "tried to recreate miner")

	Cache.DeleteMiner(accountID)
	minerFromDB := modelx.FirstOrCreateMiner(accountID)

	assert.Equal(t, "josef", miner.Name, "name wrong (cache)")
	assert.Equal(t, "23BT-2222-JCMR-22222", miner.Address, "address wrong (cache)")
	assert.Equal(t, accountID, miner.ID, "id wrong (cache)")

	assert.Equal(t, miner.Name, minerFromDB.Name, "name wrong (db)")
	assert.Equal(t, miner.ID, minerFromDB.ID, "id wrong (db)")
	assert.Equal(t, miner.Address, minerFromDB.Address, "address wrong (db)")

	modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner.ID)
}

func TestRemoveDeadlineParams(t *testing.T) {
	m := &Miner{DeadlinesParams: map[uint64]*DeadlineParams{999: &DeadlineParams{BaseTarget: 3, Deadline: 4}}}
	m.removeDeadlineParams(1)
	assert.Equal(t, 0.0, m.WeightedDeadlineSum, "removed not existing deadline params")
	m.removeDeadlineParams(999)
	assert.Equal(t, -12.0, m.WeightedDeadlineSum, "removed not existing deadline params")
}

func TestAddDeadlineParams(t *testing.T) {
	m := &Miner{DeadlinesParams: make(map[uint64]*DeadlineParams)}
	m.addDeadlineParams()
	assert.Equal(t, 0.0, m.WeightedDeadlineSum, "added not existing deadline params")
	m.CurrentDeadlineParams = &DeadlineParams{BaseTarget: 6, Deadline: 7}
	m.addDeadlineParams()
	assert.Equal(t, 42.0, m.WeightedDeadlineSum, "did not add existing deadline params")
}

func TestCalculateEEPS(t *testing.T) {
	m := Miner{
		DeadlinesParams:     map[uint64]*DeadlineParams{1: nil, 2: nil, 3: nil, 4: nil, 5: nil, 6: nil},
		WeightedDeadlineSum: 12739871461560141.4}
	assert.Equal(t, 1.4464712184653721e-05, m.CalculateEEPS())
}

/*
1. New Block
2. "Old" New Block
3. Better Deadline
4. Worse Deadline
  - current block
  - old block
5. Fast Block
*/
func TestUpdateOrCreateNonceSubmission(t *testing.T) {
	currentHeight := Cache.CurrentBlock().Height
	walletHandlerMock.On("GetGenerationTime", currentHeight).Return(int32(1), nil)

	type submission struct {
		height           uint64
		expCurrentHeight uint64

		nonce uint64

		deadline           uint64
		expCurrentDeadline uint64

		baseTarget uint64
		genSig     string

		expWeightedDeadlineSum float64
	}

	miner := Cache.GetMiner(243989817010793960)

	// New Block
	s := submission{
		height:                 currentHeight + 2,
		expCurrentHeight:       currentHeight + 2,
		nonce:                  1,
		deadline:               100,
		expCurrentDeadline:     100,
		baseTarget:             15,
		genSig:                 sampleGenSig,
		expWeightedDeadlineSum: 5.05988872415e+11}

	submissions := []submission{}
	for i := 0; i < 5; i++ {
		submissions = append(submissions, s)
	}

	// Better Deadline
	submissions[1].deadline = 5
	submissions[1].expCurrentDeadline = 5

	// Worse Deadline
	submissions[2].deadline = 6
	submissions[2].expCurrentDeadline = 5

	// Old New Block
	submissions[3].height--
	submissions[3].deadline = 200
	submissions[3].expCurrentDeadline = 5
	submissions[3].expWeightedDeadlineSum = 5.05593286595e+11

	// Old Block better deadline
	submissions[4].height--
	submissions[4].deadline = 99
	submissions[4].expCurrentDeadline = 5
	submissions[4].expWeightedDeadlineSum = 5.0559328508e+11

	for i, s := range submissions {
		walletHandlerMock.On("GetGenerationTime", s.height).Return(int32(s.height), nil)

		err := modelx.UpdateOrCreateNonceSubmission(
			miner, s.height, s.deadline, s.nonce, s.baseTarget, s.genSig)

		if !assert.Nil(t, err) {
			continue
		}
		assert.Equal(t, s.expCurrentDeadline, miner.CurrentDeadline(), "deadline set wrong", i)
		assert.Equal(t, s.expCurrentHeight, miner.CurrentBlockHeight(), "curentBlockHeight wrong", i)
		assert.Equal(t, s.expWeightedDeadlineSum, miner.WeightedDeadlineSum,
			"WeightedDeadlineSum wrong", i)
	}

	slow, exists := Cache.WasSlowBlock(currentHeight)
	assert.True(t, exists, "did not add block to cache")
	assert.False(t, slow, "did not mark block as fast")

	for _, s := range submissions {
		modelx.db.MustExec("DELETE FROM block WHERE height = ?", s.height)
	}

	modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner.ID)
}

func TestGetAVGNetDiff(t *testing.T) {
	assert.Equal(t, 250000.0, modelx.GetAVGNetDiff(0), "netDiff default wrong")
	assert.Equal(t, 333045.039, modelx.GetAVGNetDiff(100), "netDiff wrong (1)")
	assert.Equal(t, 344765.2544, modelx.GetAVGNetDiff(200), "netDiff wrong (2)")
}

func TestUpdateBestNonceSubmission(t *testing.T) {
	height := uint64(493731)
	modelx.UpdateBestSubmission(3685541669762741899, height)

	var bestNonceSubmissionID int64
	err := modelx.db.Get(&bestNonceSubmissionID,
		"SELECT best_nonce_submission_id FROM block WHERE height = ?", height)
	if assert.Nil(t, err, nil) {
		assert.Equal(t, bestNonceSubmissionID, int64(125506))
	}

	modelx.UpdateBestSubmission(13517851317125621367, height)
	err = modelx.db.Get(&bestNonceSubmissionID,
		"SELECT best_nonce_submission_id FROM block WHERE height = ?", height)
	if assert.Nil(t, err, nil) {
		assert.Equal(t, bestNonceSubmissionID, int64(125513))
	}
}

func TestCleanDB(t *testing.T) {
	currentHeight := Cache.CurrentBlock().Height

	heightToDelete := currentHeight - 3000 - 1
	heightNotToDelete := currentHeight - 3000 + 1

	minerToDelete := uint64(13371337)
	minerNotToDelete := uint64(13371338)

	// miner that should get deleted
	modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 500, "herscht")`, minerToDelete)
	modelx.db.MustExec(`INSERT INTO miner (id) VALUES (?)`, minerToDelete)
	modelx.db.MustExec(`INSERT INTO nonce_submission
                (miner_id, block_height, deadline, nonce)
                VALUES (?, ?, 1, 1)`, minerToDelete, heightToDelete)

	// miner that should not get deleted
	modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 600, "bold")`, minerNotToDelete)
	modelx.db.MustExec(`INSERT INTO miner (id) VALUES (?)`, minerNotToDelete)
	modelx.db.MustExec(`INSERT INTO nonce_submission
                (miner_id, block_height, deadline, nonce)
                VALUES (?, ?, 1, 1)`, minerNotToDelete, heightNotToDelete)

	modelx.CleanDB()

	var blockCount int
	modelx.db.Get(&blockCount, "SELECT COUNT(*) FROM block WHERE height = ?", heightToDelete)
	assert.Equal(t, 0, blockCount, "block is not deleted")

	var herschtID uint64
	modelx.db.Get(&herschtID, "SELECT id FROM account WHERE id = 13371337")
	assert.Equal(t, uint64(0), herschtID, "herscht account didn't get deleted")

	var boldID uint64
	modelx.db.Get(&boldID, "SELECT id FROM account WHERE id = 13371338")
	assert.Equal(t, uint64(13371338), boldID, "bold account got deleted")

	var feeAccountPending int64
	modelx.db.Get(&feeAccountPending, "SELECT pending FROM account WHERE id = ?", Cfg.FeeAccountID)
	assert.Equal(t, int64(38207000500), feeAccountPending, "didn't transfer pending to poolFeeAccount")

	modelx.db.MustExec("DELETE FROM account WHERE id = ?", minerNotToDelete)
	modelx.db.MustExec("DELETE FROM nonce_submission WHERE miner_id = ?", minerNotToDelete)
}

func TestGetBestNonceSubmission(t *testing.T) {
	timeDummy := time.Now()
	ns, err := modelx.GetBestNonceSubmissionOnBlock(493747)
	ns.RoundStart = timeDummy
	if assert.Nil(t, err) {
		assert.Equal(t, NonceSubmission{
			Nonce:      58101653053,
			Deadline:   6244,
			MinerID:    13517851317125621367,
			Height:     493747,
			Name:       "",
			Address:    "5BMR-6VXX-6B4G-DGA8R",
			RoundStart: timeDummy,
		}, *ns, "best nonce submission wrong")
	}
}

/*
1. min payout
  - custom
  - default
2. time
  - weekly
  - now
  - daily
*/
func TestPayout(t *testing.T) {
	type payoutTest struct {
		accountID         uint64
		pending           int64
		minPayoutValue    *int64
		nextPayoutDate    *time.Time
		expNextPayoutDate *time.Time
		payoutInterval    *string
	}

	now := time.Now()
	weeklyStr := "weekly"
	dailyStr := "daily"
	nowStr := "now"
	minPayoutValue := int64(10000000000)
	// TODO: add failed transaction tests
	expNextDaily := now.Add(24 * time.Hour)
	expNextWeekly := now.Add(7 * 24 * time.Hour)
	payoutTests := []payoutTest{
		payoutTest{
			accountID: 277277121478963799,
			pending:   251000000000},
		payoutTest{
			accountID:      1185799414684070507,
			pending:        101000000000,
			minPayoutValue: &minPayoutValue},
		payoutTest{
			accountID:      2906792196565182020,
			pending:        2000000000,
			nextPayoutDate: &now,
			payoutInterval: &nowStr},
		payoutTest{
			accountID:         13517851317125621367,
			pending:           3000000000,
			nextPayoutDate:    &now,
			expNextPayoutDate: &expNextDaily,
			payoutInterval:    &dailyStr},
		payoutTest{
			accountID:         3613974708568075609,
			pending:           4000000000,
			nextPayoutDate:    &now,
			expNextPayoutDate: &expNextWeekly,
			payoutInterval:    &weeklyStr}}

	for _, test := range payoutTests {
		walletHandlerMock.On("SendPayment", test.accountID, test.pending-Cfg.TxFee).Return(
			test.accountID, nil)
		modelx.db.MustExec(`UPDATE account SET
                                      pending = ?,
                                      min_payout_value = ?,
                                      next_payout_date = ?,
                                      payout_Interval = ?
                                    WHERE id = ?`,
			test.pending, test.minPayoutValue, test.nextPayoutDate, test.payoutInterval, test.accountID)
	}

	// ignore all other payment calls
	walletHandlerMock.On("SendPayment", mock.AnythingOfType("uint64"), mock.AnythingOfType("int64")).Return(
		uint64(0), errors.New("payment failed"))

	modelx.Payout()

	type transaction struct {
		ID          uint64
		Amount      int64
		RecipientID uint64 `db:"recipient_id"`
		Created     time.Time
	}
	for _, test := range payoutTests {
		pending := int64(-1)
		err := modelx.db.Get(&pending, "SELECT pending FROM account WHERE id = ?", test.accountID)
		if assert.Nil(t, err) {
			assert.Equal(t, int64(0), pending, "updated pending")
		}

		tx := transaction{}
		err = modelx.db.Get(&tx, `SELECT * FROM transaction WHERE id = ?`,
			test.accountID)
		if assert.Nil(t, err) {
			assert.Equal(t, tx.ID, test.accountID, "tx id wrong")
			assert.Equal(t, tx.Amount, test.pending-Cfg.TxFee, "tx amount wrong")
			assert.Equal(t, tx.RecipientID, test.accountID, "recipient id amount wrong")
		}

		// check cache for this account
		if test.accountID == uint64(13517851317125621367) {
			miner := Cache.GetMiner(test.accountID)
			if assert.NotNil(t, miner) {
				assert.Equal(t, int64(1086735605741493440), miner.Pending, "didn't reset pending")
			}
		}

		if test.expNextPayoutDate != nil {
			var nextPayoutDate time.Time
			err := modelx.db.Get(&nextPayoutDate, "SELECT next_payout_date FROM account WHERE id = ?",
				test.accountID)
			if assert.Nil(t, err, "", test.accountID) {
				assert.Equal(t, test.expNextPayoutDate.Year(), nextPayoutDate.Year(), "year wrong")
				assert.Equal(t, test.expNextPayoutDate.Month(), nextPayoutDate.Month(), "month wrong")
				assert.Equal(t, test.expNextPayoutDate.Day(), nextPayoutDate.Day(), "day wrong")
				assert.Equal(t, test.expNextPayoutDate.Hour(), nextPayoutDate.Hour(), "hour wrong")
			}
		}
	}
}

func TestRereadMinerNames(t *testing.T) {
	accountID := uint64(13517851317125621367)
	expName := "rico666"
	walletHandlerMock.On("GetAccountInfo", accountID).Return(&wallet.AccountInfo{Name: expName}, nil)
	walletHandlerMock.On("GetAccountInfo", mock.AnythingOfType("uint64")).Return(nil, errors.New(""))

	modelx.RereadMinerNames()

	var newName string
	err := modelx.db.Get(&newName, "SELECT name FROM account WHERE id = ?", accountID)
	if assert.Nil(t, err) {
		assert.Equal(t, expName, newName, "new name not in db")
	}

	miner := Cache.GetMiner(accountID)
	if assert.NotNil(t, miner) {
		assert.Equal(t, expName, miner.Name, "new name not in cahe")
	}

}

func TestSetMinPayout(t *testing.T) {
	type setMinPayoutTest struct {
		accountID         uint64
		pending           int64
		msg               string
		expNextPayoutDate *time.Time
		expPayoutInterval *string
		expMinPayoutValue *int64
	}

	now := time.Now()
	nextWeekly := now.Add(7 * 24 * time.Hour)
	nextDaily := now.Add(24 * time.Hour)
	nowStr := "now"
	weeklyStr := "weekly"
	dailyStr := "daily"
	ten := int64(10)
	twenty := int64(20)
	setMinPayoutTests := []setMinPayoutTest{
		setMinPayoutTest{
			accountID:         315567,
			pending:           100000000,
			msg:               weeklyStr,
			expPayoutInterval: &weeklyStr,
			expNextPayoutDate: &nextWeekly},
		setMinPayoutTest{
			accountID: 415567,
			pending:   100000000 - 1,
			msg:       weeklyStr},
		setMinPayoutTest{
			accountID:         315568,
			pending:           200000000,
			msg:               nowStr,
			expPayoutInterval: &nowStr,
			expNextPayoutDate: &now},
		setMinPayoutTest{
			accountID: 415568,
			pending:   200000000 - 1,
			msg:       nowStr},
		setMinPayoutTest{
			accountID:         315579,
			pending:           300000000,
			msg:               dailyStr,
			expPayoutInterval: &dailyStr,
			expNextPayoutDate: &nextDaily},
		setMinPayoutTest{
			accountID: 415579,
			pending:   300000000 - 1,
			msg:       dailyStr},
		setMinPayoutTest{
			accountID:         315570,
			pending:           400000000,
			msg:               "10",
			expMinPayoutValue: &ten},
		setMinPayoutTest{
			accountID: 415570,
			pending:   400000000 - 1,
			msg:       "10"},
		setMinPayoutTest{
			accountID:         315571,
			pending:           400000000,
			msg:               "20.0",
			expMinPayoutValue: &twenty},
		setMinPayoutTest{
			accountID: 415573,
			pending:   400000000 - 1,
			msg:       "20.0"},
		setMinPayoutTest{
			accountID: 315572,
			pending:   500000000,
			msg:       "lol"},
	}

	msgOf := make(map[uint64]string)
	for _, test := range setMinPayoutTests {
		msgOf[test.accountID] = test.msg
		modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, ?, ?)`, test.accountID, test.pending, test.accountID)
	}

	modelx.SetMinPayout(msgOf)

	type payoutInfo struct {
		MinPayoutValue *int64     `db:"min_payout_value"`
		PayoutInterval *string    `db:"payout_interval"`
		NextPayoutDate *time.Time `db:"next_payout_date"`
	}

	sql := "SELECT min_payout_value, payout_interval, next_payout_date FROM account WHERE id = ?"
	for _, test := range setMinPayoutTests {
		var payoutInfo payoutInfo
		err := modelx.db.Get(&payoutInfo, sql, test.accountID)
		if !assert.Nil(t, err) {
			continue
		}

		if test.expMinPayoutValue == nil {
			assert.Equal(t, test.expMinPayoutValue, payoutInfo.MinPayoutValue, "min payout value wrong")
		} else if assert.NotNil(t, payoutInfo.MinPayoutValue) {
			assert.Equal(t, *test.expMinPayoutValue, *payoutInfo.MinPayoutValue, "min payout value wrong")
		}

		if test.expPayoutInterval == nil {
			assert.Equal(t, test.expPayoutInterval, payoutInfo.PayoutInterval, "payout interval wrong")
		} else if assert.NotNil(t, payoutInfo.PayoutInterval) {
			assert.Equal(t, *test.expPayoutInterval, *payoutInfo.PayoutInterval, "payout interval wrong")
		}

		if test.expNextPayoutDate == nil {
			assert.Equal(t, test.expNextPayoutDate, payoutInfo.NextPayoutDate, "next payout date wrong")
		} else if assert.NotNil(t, payoutInfo.NextPayoutDate, "", test.accountID) {
			assert.Equal(t, test.expNextPayoutDate.Year(), payoutInfo.NextPayoutDate.Year(),
				"next payout date wrong")
			assert.Equal(t, test.expNextPayoutDate.Month(), payoutInfo.NextPayoutDate.Month(),
				"next payout date wrong")
			assert.Equal(t, test.expNextPayoutDate.Day(), payoutInfo.NextPayoutDate.Day(),
				"next payout date wrong")
			assert.Equal(t, test.expNextPayoutDate.Hour(), payoutInfo.NextPayoutDate.Hour(),
				"next payout date wrong")
		}
	}

	// TODO: second time
}

func TestWeightDeadline(t *testing.T) {
	assert.Equal(t, 2.2290464e+07, weightDeadline(1337, 16672))
}

func TestEeps(t *testing.T) {
	assert.Equal(t, 0.0, eeps(13, 0.0))
	assert.Equal(t, 0.0, eeps(0, 13.0))
	assert.Equal(t, 209.1678509540103, eeps(7, 1234567890))
}

func TestRound(t *testing.T) {
	assert.Equal(t, int64(5), round(5.49))
	assert.Equal(t, int64(-5), round(-5.49))
	assert.Equal(t, int64(5), round(4.5))
	assert.Equal(t, int64(-5), round(-4.5))
	assert.Equal(t, int64(0), round(0))
}
