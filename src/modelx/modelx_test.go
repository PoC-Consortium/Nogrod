// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package modelx

import (
	. "config"
	"container/list"
	"database/sql"
	"mocks"
	"testing"
	"time"
	"wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type modelxTestSuite struct {
	suite.Suite
	modelx *Modelx
}

var walletHandlerMock mocks.WalletHandler

func init() {
	InitCache()
	LoadConfig()
}

func (suite *modelxTestSuite) SetupTest() {
	InitCache()
	walletHandlerMock.On("GetMiningInfo").Return(&wallet.MiningInfo{
		Height:              9000000,
		BaseTarget:          1176576,
		GenerationSignature: "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9"}, nil)
	suite.modelx = NewModelX(&walletHandlerMock)
}

func (suite *modelxTestSuite) TearDownSuite() {
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", Cfg.FeeAccountID)
}

func (suite *modelxTestSuite) TestNewModel() {
	assert.NotNil(suite.T(), suite.modelx, "creating model failed")
}

func newBlock(suite *modelxTestSuite, height uint64) bool {
	baseTarget := uint64(1037678)
	genSig := "e7f3694df0ee08482bfdc9a8f606ad550161de7b0ef62f9fcb88e766205af075"
	_, err := suite.modelx.NewBlock(baseTarget, genSig, height)

	return assert.Nil(suite.T(), err, "err was not nil")
}

func (suite *modelxTestSuite) TestLoadCurrentBlock() {
	assert.True(suite.T(), suite.modelx.loadCurrentBlock(), "loading existing block should return true")

	suite.modelx.loadCurrentBlock()
	block := Cache.CurrentBlock()

	assert.Equal(suite.T(), uint64(9000000), block.Height, "Height wrong")
	assert.Equal(suite.T(), uint64(1176576), block.BaseTarget, "BaseTarget wrong")
	assert.Equal(suite.T(), uint32(3125), block.Scoop, "Scoop wrong")
	assert.Equal(suite.T(), "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9",
		block.GenerationSignature, "GenerationSignature wrong")

	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", 9000000)
}

func (suite *modelxTestSuite) TestCleanDB() {
	// block that should not get deleted
	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature)
                VALUES  (?, ?, ?, ?)`,
		5001, 1, 1, "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9")

	// block that should get deleted
	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature)
                VALUES  (?, ?, ?, ?)`,
		1, 1, 1, "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9")

	// miner that should get deleted
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (13371337, 500, "herscht")`)
	suite.modelx.db.MustExec(`INSERT INTO miner (id) VALUES (13371337)`)

	// miner that should not get deleted
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (13371338, 600, "bold")`)
	suite.modelx.db.MustExec(`INSERT INTO miner (id) VALUES (13371338)`)
	suite.modelx.db.MustExec(`INSERT INTO nonce_submission
                (miner_id, block_height, deadline, nonce)
                VALUES (13371338, 5001, 1, 1)`)

	Cache.StoreCurrentBlock(Block{Height: 5001})
	suite.modelx.CleanDB()

	var blockCount int
	suite.modelx.db.Get(&blockCount, "SELECT COUNT(*) FROM block WHERE height = 1")
	assert.Equal(suite.T(), 0, blockCount, "block is not deleted")

	var herschtID uint64
	suite.modelx.db.Get(&herschtID, "SELECT id FROM account WHERE id = 13371337")
	assert.Equal(suite.T(), uint64(0), herschtID, "herscht account didn't get deleted")

	var boldID uint64
	suite.modelx.db.Get(&boldID, "SELECT id FROM account WHERE id = 13371338")
	assert.Equal(suite.T(), uint64(13371338), boldID, "bold account got deleted")

	var feeAccountPending int64
	suite.modelx.db.Get(&feeAccountPending, "SELECT pending FROM account WHERE id = ?", Cfg.FeeAccountID)
	assert.Equal(suite.T(), int64(500), feeAccountPending, "didn't transfer pending to poolFeeAccount")

	suite.modelx.db.MustExec("DELETE FROM block WHERE height = 5001")
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = 13371338")
}

func (suite *modelxTestSuite) TestRereadMinerNames() {
	miner1 := Miner{ID: 1}

	miner2 := Miner{ID: 2}

	Cache.LoadOrStoreMiner(&miner1)
	Cache.LoadOrStoreMiner(&miner2)

	accountSQL := `INSERT INTO account (id, address, name) VALUES(?, ?, ?)`
	suite.modelx.db.MustExec(accountSQL, miner2.ID, "1", "")
	suite.modelx.db.MustExec(accountSQL, miner1.ID, "2", nil)

	walletHandlerMock.On("GetAccountInfo", uint64(1)).Return(&wallet.AccountInfo{Name: "one"}, nil)
	walletHandlerMock.On("GetAccountInfo", uint64(2)).Return(&wallet.AccountInfo{Name: "two"}, nil)

	suite.modelx.RereadMinerNames()

	assert.Equal(suite.T(), "one", miner1.Name, "Name of miner1 wrong")
	assert.Equal(suite.T(), "two", miner2.Name, "Name of miner2 wrong")

	var name1, name2 string
	suite.modelx.db.Get(&name1, "SELECT name FROM account WHERE id = 1")
	suite.modelx.db.Get(&name2, "SELECT name FROM account WHERE id = 2")

	assert.Equal(suite.T(), "one", name1, "Name of miner1 in db wrong")
	assert.Equal(suite.T(), "two", name2, "Name of miner2 in db wrong")

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = 1")
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = 2")
}

func (suite *modelxTestSuite) TestGetMinerFromDB() {
	suite.modelx.getMinerFromDB(2)
	assert.Nil(suite.T(), suite.modelx.getMinerFromDB(2), "did not return nil")

	expectedMiner := Miner{
		ID:              2,
		Address:         "",
		Pending:         0,
		DeadlinesParams: list.New(),
		Name:            "bla"}

	accountSQL := `INSERT INTO account (id, address, name) VALUES(?, ?, ?)`
	suite.modelx.db.MustExec(accountSQL, expectedMiner.ID, expectedMiner.Address, expectedMiner.Name)

	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature)
                VALUES  (?, ?, ?, ?)`,
		expectedMiner.CurrentBlockHeight(), 1, 1, "")

	minerSQL := `INSERT INTO miner (id) VALUES(?)`
	suite.modelx.db.MustExec(minerSQL, expectedMiner.ID)

	miner := suite.modelx.getMinerFromDB(2)
	if assert.NotNil(suite.T(), miner, "miner should not be nil") {
		assert.Equal(suite.T(), expectedMiner.ID, miner.ID, "created miner's ID is wrong")
		assert.Equal(suite.T(), expectedMiner.Address, miner.Address, "created miner's Address is wrong")
		assert.Equal(suite.T(), expectedMiner.Name, miner.Name, "created miner's Name is wrong")
		assert.Equal(suite.T(), expectedMiner.Pending, miner.Pending, "created miner's Pending is wrong")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", expectedMiner.ID)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", expectedMiner.CurrentBlockHeight())
}

func (suite *modelxTestSuite) TestCreateMiner() {
	expectedMiner := Miner{
		ID:              67,
		Address:         "2245-2222-8M6A-22222",
		Name:            "blubber",
		DeadlinesParams: list.New(),
		Pending:         0}
	walletHandlerMock.On("GetAccountInfo", uint64(expectedMiner.ID)).Return(
		&wallet.AccountInfo{Name: expectedMiner.Name}, nil)

	miner, err := suite.modelx.createMiner(expectedMiner.ID)
	if !assert.Nil(suite.T(), err, "error was not nil") {
		return
	}

	assert.Equal(suite.T(), expectedMiner.ID, miner.ID, "created miner's ID is wrong")
	assert.Equal(suite.T(), expectedMiner.Address, miner.Address, "created miner's Address is wrong")
	assert.Equal(suite.T(), expectedMiner.Name, miner.Name, "created miner's Name is wrong")
	assert.Equal(suite.T(), expectedMiner.Pending, miner.Pending, "created miner's Pending is wrong")

	dbMiner := suite.modelx.getMinerFromDB(miner.ID)
	if assert.NotNil(suite.T(), dbMiner, "dbMiner was nil") {
		assert.Equal(suite.T(), expectedMiner.ID, dbMiner.ID, "created dbMiner's ID is wrong")
		assert.Equal(suite.T(), expectedMiner.Address, dbMiner.Address,
			"created dbMiner's Address is wrong")
		assert.Equal(suite.T(), expectedMiner.Name, dbMiner.Name, "created dbMiner's Name is wrong")
		assert.Equal(suite.T(), expectedMiner.Pending, dbMiner.Pending,
			"created dbMiner's Pending is wrong")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", expectedMiner.ID)
}

func (suite *modelxTestSuite) TestNewBlock() {
	walletHandlerMock.On("GetGenerationTime", uint64(9000000)).Return(int32(1234), nil)

	baseTarget := uint64(1037678)
	genSig := "e7f3694df0ee08482bfdc9a8f606ad550161de7b0ef62f9fcb88e766205af075"
	height := uint64(51157)
	block, err := suite.modelx.NewBlock(baseTarget, genSig, height)

	if !assert.Nil(suite.T(), err, "err was not nil") {
		return
	}

	assert.Equal(suite.T(), baseTarget, block.BaseTarget, "baseTarget is wrong")
	assert.Equal(suite.T(), height, block.Height, "height is wrong")
	assert.Equal(suite.T(), genSig, block.GenerationSignature, "generationSignature is wrong")
	assert.NotEmpty(suite.T(), block.Created, "created is empty")

	assert.Equal(suite.T(), block, Cache.CurrentBlock(), "did not store current block")

	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", height)
}

func (suite *modelxTestSuite) TestFirstOrCreateMiner() {
	miner := Miner{
		ID:              88899,
		DeadlinesParams: list.New()}

	Cache.LoadOrStoreMiner(&miner)
	assert.Equal(suite.T(), &miner, suite.modelx.FirstOrCreateMiner(miner.ID),
		"did not return cached miner")

	Cache.DeleteMiner(miner.ID)
	walletHandlerMock.On("GetAccountInfo", miner.ID).Return(&wallet.AccountInfo{Name: "josef"}, nil)
	createdMiner := suite.modelx.FirstOrCreateMiner(miner.ID)
	if !assert.NotNil(suite.T(), createdMiner, "miner is nil") {
		return
	}
	assert.Equal(suite.T(), miner.ID, createdMiner.ID, "did not create miner")

	Cache.DeleteMiner(miner.ID)
	dbMiner := suite.modelx.FirstOrCreateMiner(miner.ID)
	if !assert.NotNil(suite.T(), dbMiner, "miner is nil") {
		return
	}
	assert.Equal(suite.T(), miner.ID, createdMiner.ID, "did not get miner from db")

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner.ID)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", miner.CurrentBlockHeight())
}

func (suite *modelxTestSuite) TestUpdateOrCreateNonceSubmission() {
	miner := &Miner{
		ID:              1447,
		DeadlinesParams: list.New()}

	accountSQL := `INSERT INTO account (id, address) VALUES(?, ?)`
	suite.modelx.db.MustExec(accountSQL, miner.ID, "Peter")

	minerSQL := `INSERT INTO miner (id) VALUES(?)`
	suite.modelx.db.MustExec(minerSQL, miner.ID)

	if success := newBlock(suite, miner.CurrentBlockHeight()); !success {
		return
	}

	if success := newBlock(suite, miner.CurrentBlockHeight()+1); !success {
		return
	}

	Cache.StoreCurrentBlock(Block{Height: miner.CurrentBlockHeight() + 1})

	err := suite.modelx.UpdateOrCreateNonceSubmission(miner, miner.CurrentBlockHeight()+1, 123, 107, 11)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), uint64(123), miner.CurrentDeadline(), "deadline set wrong")
		assert.Equal(suite.T(), uint64(1), miner.CurrentBlockHeight(), "curentBlockHeight wrong")
		assert.Equal(suite.T(), uint64(11), miner.CurrentDeadlineParams.BaseTarget, "BaseTarget wrong")
		assert.Equal(suite.T(), float64(0), miner.WeightedDeadlineSum, "WeightedDeadlineSum wrong")
	}

	err = suite.modelx.UpdateOrCreateNonceSubmission(miner, miner.CurrentBlockHeight(), 23, 24, 11)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), uint64(23), miner.CurrentDeadline(), "deadline set wrong")
		assert.Equal(suite.T(), uint64(1), miner.CurrentBlockHeight(), "currentBlockHeight wrong")
		assert.Equal(suite.T(), uint64(11), miner.CurrentDeadlineParams.BaseTarget, "BaseTarget wrong")
		assert.Equal(suite.T(), float64(0), miner.WeightedDeadlineSum, "WeightedDeadlineSum wrong")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner.ID)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", miner.CurrentBlockHeight()-1)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", miner.CurrentBlockHeight())
}

func (suite *modelxTestSuite) TestUpdateBestSubmission() {
	height := uint64(9000001)
	if success := newBlock(suite, height); !success {
		return
	}

	miner := &Miner{
		ID:              1543,
		DeadlinesParams: list.New()}

	accountSQL := `INSERT INTO account (id, address) VALUES(?, ?)`
	suite.modelx.db.MustExec(accountSQL, miner.ID, "Pietro")

	minerSQL := `INSERT INTO miner (id) VALUES(?)`
	suite.modelx.db.MustExec(minerSQL, miner.ID)

	err := suite.modelx.UpdateOrCreateNonceSubmission(miner, height, 213, 65, 11)
	if assert.Nil(suite.T(), err, "error occured") {
		suite.modelx.UpdateBestSubmission(miner.ID, height)
		assert.Nil(suite.T(), err, "error shouldn't have occured")
	}
	suite.modelx.loadCurrentBlock()
	block := Cache.CurrentBlock()

	ns, err := suite.modelx.GetBestNonceSubmissionOnBlock(block.Height)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), uint64(213), ns.Deadline, "deadline wrong")
		assert.Equal(suite.T(), uint64(65), ns.Nonce, "best nonce wrong")
		assert.Equal(suite.T(), miner.ID, ns.MinerID, "miner id wrong")
		assert.Equal(suite.T(), "", ns.Name, "name wrong")
		assert.Equal(suite.T(), "Pietro", ns.Address, "address wrong")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner.ID)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", height)
}

func (suite *modelxTestSuite) TestSetMinPayout() {
	msgOf := make(map[uint64]string)
	account1 := uint64(315567)
	account2 := uint64(315568)
	account3 := uint64(315569)
	account4 := uint64(315579)
	account5 := uint64(315570)
	account6 := uint64(315571)

	msgOf[account1] = "weekly"
	msgOf[account2] = "now"
	msgOf[account3] = "daily"
	msgOf[account4] = "10"
	msgOf[account5] = "20.0"
	msgOf[account6] = "lol"

	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 100000000, ?)`, account1, account1)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 200000000, ?)`, account2, account2)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 300000000, ?)`, account3, account3)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 400000000, ?)`, account4, account4)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 400000000, ?)`, account5, account5)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, 500000000, ?)`, account6, account6)

	suite.modelx.SetMinPayout(msgOf)

	type MinPayoutInfo struct {
		MinPayoutValue sql.NullInt64  `db:"min_payout_value"`
		PayoutInterval sql.NullString `db:"payout_interval"`
		NextPayoutDate *time.Time     `db:"next_payout_date"`
	}

	checkMinPayout := func() {
		sql := "SELECT min_payout_value, payout_interval, next_payout_date FROM account WHERE id = ?"
		var mpi1, mpi2, mpi3, mpi4, mpi5, mpi6 MinPayoutInfo
		err := suite.modelx.db.Get(&mpi1, sql, account1)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.False(suite.T(), mpi1.MinPayoutValue.Valid, "MinPayoutValue was set")
			assert.NotNil(suite.T(), mpi1.NextPayoutDate, "NextPayoutDelay was nil")
			assert.True(suite.T(), mpi1.PayoutInterval.Valid, "PayoutInterval was not set")
			assert.Equal(suite.T(), "weekly", mpi1.PayoutInterval.String, "PayoutInterval was set")
		}
		err = suite.modelx.db.Get(&mpi2, sql, account2)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.False(suite.T(), mpi2.MinPayoutValue.Valid, "MinPayoutValue was set")
			assert.NotNil(suite.T(), mpi2.NextPayoutDate, "NextPayoutDelay was nil")
			assert.True(suite.T(), mpi2.PayoutInterval.Valid, "PayoutInterval was not set")
		}
		err = suite.modelx.db.Get(&mpi3, sql, account3)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.False(suite.T(), mpi3.MinPayoutValue.Valid, "MinPayoutValue was set")
			assert.NotNil(suite.T(), mpi3.NextPayoutDate, "NextPayoutDelay was nil")
			assert.True(suite.T(), mpi3.PayoutInterval.Valid, "PayoutInterval was not set")
			assert.Equal(suite.T(), "daily", mpi3.PayoutInterval.String, "PayoutInterval was set")
		}
		err = suite.modelx.db.Get(&mpi4, sql, account4)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.True(suite.T(), mpi4.MinPayoutValue.Valid, "MinPayoutValue was not set")
			assert.Equal(suite.T(), int64(10), mpi4.MinPayoutValue.Int64, "minPayout was not set")
			assert.Nil(suite.T(), mpi4.NextPayoutDate, "NextPayoutDelay was not nil")
			assert.False(suite.T(), mpi4.PayoutInterval.Valid, "PayoutInterval was set")
		}
		err = suite.modelx.db.Get(&mpi5, sql, account5)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.True(suite.T(), mpi5.MinPayoutValue.Valid, "MinPayoutValue was not set")
			assert.Equal(suite.T(), int64(20), mpi5.MinPayoutValue.Int64, "minPayout was not set")
			assert.Nil(suite.T(), mpi5.NextPayoutDate, "NextPayoutDelay was not nil")
			assert.False(suite.T(), mpi5.PayoutInterval.Valid, "PayoutInterval was set")
		}
		err = suite.modelx.db.Get(&mpi6, sql, account6)
		if assert.Nil(suite.T(), err, "error occured") {
			assert.False(suite.T(), mpi6.MinPayoutValue.Valid, "MinPayoutValue was set")
			assert.Nil(suite.T(), mpi6.NextPayoutDate, "NextPayoutDelay was not nil")
			assert.False(suite.T(), mpi6.PayoutInterval.Valid, "PayoutInterval was set")
		}
	}

	checkMinPayout()

	suite.modelx.SetMinPayout(msgOf)

	// mix the messages
	msgOf[account1] = "lol"
	msgOf[account2] = "daily"
	msgOf[account3] = "weekly"
	msgOf[account4] = "20.0"
	msgOf[account5] = "10"
	msgOf[account6] = "now"

	// we don't expect any changes, since the accounts don't have enough
	// funds now
	checkMinPayout()

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account1)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account2)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account3)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account4)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account5)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", account6)
}

func (suite *modelxTestSuite) TestPayout() {
	// normal over threshold
	miner1 := &Miner{
		ID:      155676632,
		Pending: 12345678900000}
	walletHandlerMock.On("SendPayment", miner1.ID, miner1.Pending-Cfg.TxFee).Return(
		uint64(1), nil)
	Cache.LoadOrStoreMiner(miner1)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, ?, ?)`, miner1.ID, miner1.Pending, miner1.ID)
	suite.modelx.Payout()

	var miner1Pending int64
	err := suite.modelx.db.Get(&miner1Pending, "SELECT pending FROM account WHERE id = ?", miner1.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(0), miner1.Pending, "no pending reset in cache")
		assert.Equal(suite.T(), int64(0), miner1Pending, "no pending reset in db")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner1.ID)
	suite.modelx.db.MustExec("DELETE FROM transaction WHERE id = 1")

	// normal under threshold
	miner2 := &Miner{
		ID:      155676633,
		Pending: 1000000000}
	Cache.LoadOrStoreMiner(miner1)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address)
                VALUES (?, ?, ?)`, miner2.ID, miner2.Pending, miner2.ID)
	suite.modelx.Payout()

	var miner2Pending int64
	err = suite.modelx.db.Get(&miner2Pending, "SELECT pending FROM account WHERE id = ?", miner2.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(1000000000), miner2.Pending, "pending reset in cache")
		assert.Equal(suite.T(), int64(1000000000), miner2Pending, "pending reset in db")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner2.ID)
	suite.modelx.db.MustExec("DELETE FROM transaction WHERE id = 1")

	// date due
	miner3 := &Miner{
		ID:      155676638,
		Pending: 9876543211123}
	walletHandlerMock.On("SendPayment", miner3.ID, miner3.Pending-Cfg.TxFee).Return(
		uint64(3), nil)
	Cache.LoadOrStoreMiner(miner3)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address, next_payout_date, payout_interval)
                VALUES (?, ?, ?, ?, ?)`, miner3.ID, miner3.Pending, miner3.ID, time.Now(), "weekly")
	suite.modelx.Payout()

	var miner3Pending int64
	err = suite.modelx.db.Get(&miner3Pending, "SELECT pending FROM account WHERE id = ?", miner3.ID)

	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(0), miner3.Pending, "no pending reset in cache")
		assert.Equal(suite.T(), int64(0), miner3Pending, "no pending reset in db")
	}

	var nextPayoutDate time.Time
	err = suite.modelx.db.Get(&nextPayoutDate, "SELECT next_payout_date FROM account WHERE id = ?", miner3.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), time.Now().AddDate(0, 0, 7).Day(), nextPayoutDate.Day(),
			"no reset of nextPayoutDate")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner3.ID)
	suite.modelx.db.MustExec("DELETE FROM transaction WHERE id = 3")

	// date not due
	miner4 := &Miner{
		ID:      155672638,
		Pending: 10000000000000}
	walletHandlerMock.On("SendPayment", miner4.ID, miner4.Pending-Cfg.TxFee).Return(
		uint64(4), nil)
	Cache.LoadOrStoreMiner(miner4)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address, next_payout_date)
                VALUES (?, ?, ?, ?)`, miner4.ID, miner4.Pending, miner4.ID, time.Now().AddDate(0, 0, 1))
	suite.modelx.Payout()

	var miner4Pending int64
	err = suite.modelx.db.Get(&miner4Pending, "SELECT pending FROM account WHERE id = ?", miner4.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(10000000000000), miner4.Pending, "no pending reset in cache")
		assert.Equal(suite.T(), int64(10000000000000), miner4Pending, "no pending reset in db")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner4.ID)
	suite.modelx.db.MustExec("DELETE FROM transaction WHERE id = 4")

	// custom threshold over
	miner5 := &Miner{
		ID:      152672638,
		Pending: 100000001}
	walletHandlerMock.On("SendPayment", miner5.ID, miner5.Pending-Cfg.TxFee).Return(
		uint64(5), nil)
	Cache.LoadOrStoreMiner(miner5)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address, min_payout_value)
                VALUES (?, ?, ?, ?)`, miner5.ID, miner5.Pending, miner5.ID, 1)
	suite.modelx.Payout()

	var miner5Pending int64
	err = suite.modelx.db.Get(&miner5Pending, "SELECT pending FROM account WHERE id = ?", miner5.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(0), miner5.Pending, "no pending reset in cache")
		assert.Equal(suite.T(), int64(0), miner5Pending, "no pending reset in db")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner5.ID)
	suite.modelx.db.MustExec("DELETE FROM transaction WHERE id = 5")

	// custom threshold under
	miner6 := &Miner{
		ID:      152672038,
		Pending: 100000001}
	Cache.LoadOrStoreMiner(miner6)
	suite.modelx.db.MustExec(`INSERT INTO account
                (id, pending, address, min_payout_value)
                VALUES (?, ?, ?, ?)`, miner6.ID, miner6.Pending, miner6.ID, 2)
	suite.modelx.Payout()

	var miner6Pending int64
	err = suite.modelx.db.Get(&miner6Pending, "SELECT pending FROM account WHERE id = ?", miner6.ID)
	if assert.Nil(suite.T(), err, "error occured") {
		assert.Equal(suite.T(), int64(100000001), miner6.Pending, "no pending reset in cache")
		assert.Equal(suite.T(), int64(100000001), miner6Pending, "no pending reset in db")
	}

	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", miner6.ID)
}

func (suite *modelxTestSuite) TestGetAVGNetDiff() {
	assert.Equal(suite.T(), 250000.0, suite.modelx.GetAVGNetDiff(0), "netDiff default wrong")

	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature)
                VALUES  (?, ?, ?, ?)`,
		3000, 2012, 1, "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9")

	assert.Equal(suite.T(), 31096.8613, suite.modelx.GetAVGNetDiff(1), "netDiff wrong (1)")
	assert.Equal(suite.T(), 31096.8613, suite.modelx.GetAVGNetDiff(2), "netDiff wrong (2)")

	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature)
                VALUES  (?, ?, ?, ?)`,
		3001, 3000, 1, "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9")

	assert.Equal(suite.T(), 46526.8616, suite.modelx.GetAVGNetDiff(1), "netDiff wrong (3)")
	assert.Equal(suite.T(), 46526.8616, suite.modelx.GetAVGNetDiff(2), "netDiff wrong (4)")

	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", 3000)
	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", 3001)
}

func (suite *modelxTestSuite) TestGetRecentlyWonBlocks() {
	assert.Empty(suite.T(), suite.modelx.GetRecentlyWonBlocks(), "got some wom blocks")

	height := uint64(3008)
	baseTarget := uint64(2012)
	scoop := uint32(1)
	genSig := "35844ab83e21851b38340cd7e8fc96b8bc139c132759ce3de1fcb616d888f2c9"
	reward := int64(777700000000)
	minerID := uint64(1009)
	accountName := "rico666"
	accountAddress := "somewhere"
	bestNonceSubmissionID := int64(1778)
	deadline := uint64(13)
	nonce := uint64(56)

	suite.modelx.db.MustExec(`INSERT INTO account
                (id, name, address)
                VALUES  (?, ?, ?)`,
		minerID, accountName, accountAddress)

	suite.modelx.db.MustExec(`INSERT INTO block
                (height, base_target, scoop, generation_signature, reward, winner_id)
                VALUES  (?, ?, ?, ?, ?, ?)`,
		height, baseTarget, scoop, genSig, reward, minerID)

	suite.modelx.db.MustExec(`INSERT INTO nonce_submission
                (id, deadline, miner_id, nonce, block_height)
                VALUES  (?, ?, ?, ?, ?)`,
		bestNonceSubmissionID, deadline, minerID, nonce, height)

	suite.modelx.db.MustExec("UPDATE block SET best_nonce_submission_id = ? WHERE height = ?",
		bestNonceSubmissionID, height)

	wonBlocks := suite.modelx.GetRecentlyWonBlocks()
	if assert.Equal(suite.T(), 1, len(wonBlocks), "more or less than 1 won block") {
		assert.Equal(suite.T(), accountName, wonBlocks[0].WinnerName, "WinnerName wrong")
		assert.Equal(suite.T(), accountAddress, wonBlocks[0].WinnerAddress, "WinnerAddress wrong")
		assert.Equal(suite.T(), minerID, wonBlocks[0].WinnerID, "WinnerID wrong")
		assert.Equal(suite.T(), deadline, wonBlocks[0].Deadline, "Deadline wrong")
		assert.Equal(suite.T(), baseTarget, wonBlocks[0].BaseTarget, "BaseTarget wrong")
		assert.Equal(suite.T(), height, wonBlocks[0].Height, "Height wrong")
		assert.Equal(suite.T(), float64(reward/100000000), wonBlocks[0].Reward, "Reward wrong")
	}

	suite.modelx.db.MustExec("DELETE FROM block WHERE height = ?", height)
	suite.modelx.db.MustExec("DELETE FROM account WHERE id = ?", minerID)
}

func TestCurrentBlock(t *testing.T) {
	b := Block{Height: 123}
	Cache.StoreCurrentBlock(b)

	assert.Equal(t, b, Cache.CurrentBlock())
}

func TestMiner(t *testing.T) {
	assert.Nil(t, Cache.GetMiner(1), "cache returned non existing miner")

	miner := &Miner{ID: 1}

	Cache.LoadOrStoreMiner(miner)
	assert.Equal(t, miner, Cache.GetMiner(miner.ID), "did not return same miner")

	Cache.DeleteMiner(miner.ID)
	assert.Nil(t, Cache.GetMiner(1), "cache did not delete miner")

	miner1 := &Miner{ID: 1}

	miner2 := &Miner{ID: 2}

	Cache.LoadOrStoreMiner(miner1)
	Cache.LoadOrStoreMiner(miner2)

	Cache.MinerRange(func(k, v interface{}) bool {
		miner := v.(*Miner)
		miner.Lock()
		defer miner.Unlock()

		if miner.ID == 1 {
			miner.Name = "floor"
		} else {
			miner.Name = "ceiling"
		}

		return true
	})

	assert.Equal(t, miner1.Name, "floor", "set wrong name for miner1")
	assert.Equal(t, miner2.Name, "ceiling", "set wrong name for miner2")
}

func TestRewardRecipient(t *testing.T) {
	Cache.StoreRewardRecipient(1, false)
	isCorrect, stored := Cache.IsRewardRecipient(1)
	assert.Equal(t, true, stored, "wasn't stored")
	assert.Equal(t, false, isCorrect, "was correct")

	isCorrect, stored = Cache.IsRewardRecipient(2)
	assert.Equal(t, false, stored, "wasn't stored")
	assert.Equal(t, false, isCorrect, "was correct")

	Cache.StoreRewardRecipient(2, true)
	isCorrect, stored = Cache.IsRewardRecipient(2)
	assert.Equal(t, true, stored, "wasn't stored")
	assert.Equal(t, true, isCorrect, "wasn't correct")
}

func TestBestNonceSubmission(t *testing.T) {
	bs := NonceSubmission{MinerID: 123}
	Cache.StoreBestNonceSubmission(bs)

	assert.Equal(t, bs, Cache.BestNonceSubmission())
}

func TestModelxSuite(t *testing.T) {
	tests := new(modelxTestSuite)
	suite.Run(t, tests)
}
