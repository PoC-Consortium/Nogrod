// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package wallethandler

import (
	"testing"
	"time"

	. "github.com/PoC-Consortium/Nogrod/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var walletUrls = []string{"http://wallet.dev.burst-test.net:6876"}
var secretPhrase = "glad suffer red during single glow shut slam hill death lust although"

type walletTestSuite struct {
	suite.Suite
	wh *walletHandler
}

func (suite *walletTestSuite) SetupSuite() {
	LoadConfig()
	suite.wh = NewWalletHandler(walletUrls, secretPhrase, time.Second*10, false).(*walletHandler)
	assert.Equal(suite.T(), secretPhrase, suite.wh.secretPhrase, "secretPhrase isn't intialized correctly")
	assert.Equal(suite.T(), 1, len(suite.wh.wallets), "wallet count not ok")
}

func (suite *walletTestSuite) TestGetMiningInfo() {
	miningInfo, err := suite.wh.GetMiningInfo()
	if assert.Nil(suite.T(), err) {
		assert.NotEmpty(suite.T(), miningInfo.GenerationSignature, "GenerationSignature is empty")
		assert.NotEmpty(suite.T(), miningInfo.BaseTarget, "BaseTarget is empty")
		assert.NotEmpty(suite.T(), miningInfo.Height, "Height is empty")
		assert.Empty(suite.T(), miningInfo.ErrorDescription, "ErrorDescription isn't empty")
	}
}

func (suite *walletTestSuite) TestGetBlockInfo() {
	blockInfo, err := suite.wh.GetBlockInfo(74122)
	if assert.Nil(suite.T(), err) {
		assert.NotEmpty(suite.T(), blockInfo.Generator)
		assert.NotEmpty(suite.T(), blockInfo.BlockReward)
		assert.NotEmpty(suite.T(), blockInfo.TotalFeeNQT)
		assert.NotEmpty(suite.T(), blockInfo.BaseTarget)
		assert.Empty(suite.T(), blockInfo.ErrorDescription)
	}

	_, err = suite.wh.GetBlockInfo(^uint64(0))
	assert.NotNil(suite.T(), err)
}

func (suite *walletTestSuite) TestSubmitNonce() {
	// TODO: those tests won't work, because a static deadline
	// won't match the wallet's deadline
	// err := suite.wh.SubmitNonce(1337, 6418289488649374107, 10)
	// assert.Equal(suite.T(), "", err.Error())
}

func (suite *walletTestSuite) TestSendPayment() {
	_, err := suite.wh.SendPayment(133, 0)
	assert.NotNil(suite.T(), err)

	// TODO: this would send money through the test network
	txID, err := suite.wh.SendPayment(133, 1)

	assert.Nil(suite.T(), err)
	assert.NotEmpty(suite.T(), txID, "txID is empty")
}

func (suite *walletTestSuite) TestGetAccountInfo() {
	accountInfo, err := suite.wh.GetAccountInfo(10282355196851764065)
	if assert.Nil(suite.T(), err) {
		assert.NotEmpty(suite.T(), accountInfo.Name, "name is empty")
	}

	_, err = suite.wh.GetAccountInfo(0)
	assert.NotNil(suite.T(), err, "err is nil")
}

func (suite *walletTestSuite) TestWonBlock() {
	won, _, err := suite.wh.WonBlock(1, 2, 3)
	if assert.Nil(suite.T(), err) {
		assert.False(suite.T(), won, "shouldn't have won")
	}

	won, _, err = suite.wh.WonBlock(54896, 1, 3)
	if assert.Nil(suite.T(), err) {
		assert.False(suite.T(), won, "shouldn't have won")
	}

	// TODO: we need to win a block in the dev net to test this
	// won, blockInfo, err := suite.wh.WonBlock(54896, 10282355196851764065, 123575369)
	// if assert.Nil(suite.T(), err) {
	// 	assert.True(suite.T(), won, "should have won")
	// 	assert.Equal(suite.T(), blockInfo.Generator, uint64(10282355196851764065), "GeneratorID is wrong")
	// }
}

func (suite *walletTestSuite) TestGetIncomingMsgsSince() {
	date := time.Unix(0, 0)
	msgOf, err := suite.wh.GetIncomingMsgsSince(date)
	if assert.Nil(suite.T(), err) {
		assert.NotEmpty(suite.T(), msgOf, "no tx ids")
		assert.Equal(suite.T(), "10", msgOf[6418289488649374107], "msg wrong")
	}
}

func (suite *walletTestSuite) TestGetTransaction() {
	txInfo, querySuccessful, err := suite.wh.GetTransaction(1)
	assert.True(suite.T(), querySuccessful)
	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), txInfo)
	txInfo, querySuccessful, err = suite.wh.GetTransaction(16409086240127269435)
	assert.True(suite.T(), querySuccessful)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), txInfo)
}

func (suite *walletTestSuite) TestCalcOptiomalTxFee() {
	txFee, err := suite.wh.CalcOptimalTxFee(77406)
	if assert.Nil(suite.T(), err) {
		assert.Equal(suite.T(), int64(1470000), txFee)
	}
}

func TestWalletSuite(t *testing.T) {
	tests := new(walletTestSuite)
	suite.Run(t, tests)
}
