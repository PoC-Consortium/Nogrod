// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package wallet

import (
	. "config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

var walletUrls = []string{"http://176.9.47.157:6876"}
var secretPhrase = "glad suffer red during single glow shut slam hill death lust although"

type walletTestSuite struct {
	suite.Suite
	wallet *BrsWallet
}

func (suite *walletTestSuite) SetupSuite() {
	LoadConfig()
	suite.wallet = NewBrsWallet(walletUrls, secretPhrase)
	assert.Equal(suite.T(), secretPhrase, suite.wallet.secretPhrase, "secretPhrase isn't intialized correctly")
}

func (suite *walletTestSuite) TearDownSuite() {
	suite.wallet.Stop()
}

func (suite *walletTestSuite) TestGetMiningInfo() {
	miningInfo, err := suite.wallet.GetMiningInfo()
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.NotEmpty(suite.T(), miningInfo.GenerationSignature, "GenerationSignature is empty")
		assert.NotEmpty(suite.T(), miningInfo.BaseTarget, "BaseTarget is empty")
		assert.NotEmpty(suite.T(), miningInfo.Height, "Height is empty")
		assert.Empty(suite.T(), miningInfo.ErrorDescription, "ErrorDescription isn't empty")
	}
}

func (suite *walletTestSuite) TestGetConstantsInfo() {
	constantsInfo, err := suite.wallet.GetConstantsInfo()
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.NotEmpty(suite.T(), constantsInfo.GenesisBlockID, "GenesisBlockID is empty")
		assert.Empty(suite.T(), constantsInfo.ErrorDescription, "ErrorDescription isn't empty")
	}
}

func (suite *walletTestSuite) TestGetBlockInfo() {
	blockInfo, err := suite.wallet.GetBlockInfo(40000)
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.Equal(suite.T(), uint64(0x8f7dd7bcaa4f2037), blockInfo.GeneratorID, "GeneratorID incorrect")
		assert.Equal(suite.T(), int64(0x217d), blockInfo.BlockReward, "BlockReward incorrect")
		assert.Equal(suite.T(), int64(0x0), blockInfo.TotalFeeNQT, "TotalFeeNQT incorrect")
		assert.Equal(suite.T(), uint64(0x3869ad8ca), blockInfo.BaseTarget, "BaseTarget is incorrect")
		assert.Empty(suite.T(), blockInfo.ErrorDescription, "ErrorDescription isn't empty")
	}

	blockInfo, err = suite.wallet.GetBlockInfo(^uint64(0))
	if assert.NotNil(suite.T(), err, "err is nil") {
		assert.Equal(suite.T(), "Incorrect \"height\"", err.Error(), "error description incorrect")
	}
}

func (suite *walletTestSuite) TestGetRewardRecipientInfo() {
	rewardRecipientInfo, err := suite.wallet.GetRewardRecipientInfo(6418289488649374107)
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.Equal(suite.T(), uint64(0x8eb24392c2bdbb61), rewardRecipientInfo.RewardRecipientID,
			"RewardRecipientID incorrect")
	}

	rewardRecipientInfo, err = suite.wallet.GetRewardRecipientInfo(0)
	if assert.NotNil(suite.T(), err, "err is nil") {
		assert.Equal(suite.T(), "Unknown account", err.Error(), "err description incorrect")
	}
}

func (suite *walletTestSuite) TestSubmitNonce() {
	deadline, err := suite.wallet.SubmitNonce(1337, 6418289488649374107)
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.NotEmpty(suite.T(), deadline, "deadline is empty")
	}

	deadline, err = suite.wallet.SubmitNonce(1337, 1337)
	if assert.NotNil(suite.T(), err, "err isn't nil") {
		assert.Equal(suite.T(), "Passphrase does not match reward recipient", err.Error(),
			"err description incorrect")
	}
}

func (suite *walletTestSuite) TestSendPayment() {
	_, err := suite.wallet.SendPayment(133, 0)
	if assert.NotNil(suite.T(), err, "err is nil") {
		assert.Equal(suite.T(), "Incorrect \"amount\"", err.Error(), "err description incorrect")
	}

	// TODO: this would send money through the test network
	txID, err := suite.wallet.SendPayment(133, 1)

	assert.Nil(suite.T(), err, "err isn't nil")
	assert.NotEmpty(suite.T(), txID, "txID is empty")
}

func (suite *walletTestSuite) TestGetBalance() {
	balance, err := suite.wallet.GetBalance(6418289488649374107)
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.NotEmpty(suite.T(), balance, "balance is empty")
	}

	balance, err = suite.wallet.GetBalance(0)
	if assert.NotNil(suite.T(), err, "err is nil") {
		assert.Equal(suite.T(), "Unknown account", err.Error(), "error description incorrect")
	}
}

func (suite *walletTestSuite) TestGetAccountInfo() {
	accountInfo, err := suite.wallet.GetAccountInfo(10339657524823662647)
	if assert.Nil(suite.T(), err, "err isn't nil") {
		assert.NotEmpty(suite.T(), accountInfo.Name, "name is empty")
	}

	accountInfo, err = suite.wallet.GetAccountInfo(0)
	if assert.NotNil(suite.T(), err, "err is nil") {
		assert.Equal(suite.T(), "Unknown account", err.Error(), "error description incorrect")
	}
}

func (suite *walletTestSuite) TestWonBlock() {
	won, _, err := suite.wallet.WonBlock(1, 2)
	if assert.Nil(suite.T(), err, "err occured") {
		assert.False(suite.T(), won, "shouldn't have won")
	}

	won, _, err = suite.wallet.WonBlock(54896, 1)
	if assert.Nil(suite.T(), err, "err occured") {
		assert.False(suite.T(), won, "shouldn't have won")
	}

	won, blockInfo, err := suite.wallet.WonBlock(54896, 123575369)
	if assert.Nil(suite.T(), err, "err occured") {
		assert.True(suite.T(), won, "should have won")
		assert.Equal(suite.T(), blockInfo.GeneratorID, uint64(10282355196851764065), "GeneratorID is wrong")
	}
}

func (suite *walletTestSuite) TestGetIncomingMsgsSince() {
	date := time.Unix(1518220800, 0)
	msgOf, err := suite.wallet.GetIncomingMsgsSince(date)
	if assert.Nil(suite.T(), err, "err occured") {
		assert.NotEmpty(suite.T(), msgOf, "no tx ids")
		assert.Equal(suite.T(), "10", msgOf[12441003299556495598], "msg wrong")
	}
}

func TestWalletSuite(t *testing.T) {
	tests := new(walletTestSuite)
	suite.Run(t, tests)
}
