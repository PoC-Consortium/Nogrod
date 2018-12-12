package wallet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	devWalletURL  = "http://wallet.dev.burst-test.net:6876"
	realWalletURL = "https://wallet.burst.cryptoguru.org:8125"
)

var w = NewWallet(devWalletURL, 20*time.Second, false).(*wallet)
var rw = NewWallet(realWalletURL, 10*time.Second, false).(*wallet)

func TestNewWallet(t *testing.T) {
	assert.Equal(t, w.apiURL, devWalletURL+"/burst?")
}

func TestGetMiningInfo(t *testing.T) {
	res, err := w.GetMiningInfo()
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res.Height)
		assert.NotEmpty(t, res.BaseTarget)
		assert.NotEmpty(t, res.GenerationSignature)
		assert.Empty(t, res.ErrorDescription)
	}
}

func TestSubmitNonce(t *testing.T) {
	res, err := rw.SubmitNonce(&SubmitNonceRequest{
		AccountID:    6854086812727909295,
		Nonce:        10,
		SecretPhrase: "sigh forever inner appreciate fail unless second image choice pink huge control"})
	if assert.Nil(t, err) {
		assert.Equal(t, "success", res.Result)
		assert.NotEmpty(t, "deadline", res.Deadline)
		return
	}
}

func TestGetAccountsWithRewardRecipient(t *testing.T) {
	res, err := w.GetAccountsWithRewardRecipient(&GetAccountsWithRewardRecipientRequest{
		AccountID: 5658931570366906527})
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res.Recipients)
	}
}

func TestGetBlock(t *testing.T) {
	res, err := rw.GetBlock(&GetBlockRequest{Height: 471696})
	if !assert.Nil(t, err) {
		return
	}
	assert.NotEmpty(t, res.PreviousBlockHash)
	assert.NotEmpty(t, res.PayloadLength)
	assert.NotEmpty(t, res.TotalAmountNQT)
	assert.NotEmpty(t, res.GenerationSignature)
	assert.NotEmpty(t, res.Generator)
	assert.NotEmpty(t, res.GeneratorPublicKey)
	assert.NotEmpty(t, res.BaseTarget)
	assert.NotEmpty(t, res.PayloadHash)
	assert.NotEmpty(t, res.GeneratorRS)
	assert.NotEmpty(t, res.BlockReward)
	assert.NotEmpty(t, res.NextBlock)
	assert.NotEmpty(t, res.ScoopNum)
	assert.NotEmpty(t, res.BlockSignature)
	if assert.NotEmpty(t, len(res.Transactions)) {
		assert.NotEmpty(t, uint64(res.Transactions[0]))
	}
	assert.NotEmpty(t, res.Nonce)
	assert.NotEmpty(t, res.Version)
	assert.NotEmpty(t, res.PreviousBlock)
	assert.NotEmpty(t, res.Block)
	assert.NotEmpty(t, res.Height)
	assert.NotEmpty(t, res.Timestamp)
}

func TestEncodeRecipients(t *testing.T) {
	_, err := EncodeRecipients(make(map[uint64]int64))
	assert.NotNil(t, err)
	encoded, err := EncodeRecipients(map[uint64]int64{
		1: 2,
		3: 4})
	if assert.Nil(t, err) {
		assert.True(t, encoded == "1:2;3:4" || encoded == "3:4;1:2")
	}

	tooManyRecips := make(map[uint64]int64)
	for i := uint64(0); i < 65; i++ {
		tooManyRecips[i] = 1
	}
	assert.Panics(t, func() { EncodeRecipients(tooManyRecips) })
}

func TestSendMoney(t *testing.T) {
	res, err := w.SendMoney(&SendMoneyRequest{
		Recipient:    6418289488649374107,
		AmountNQT:    1,
		FeeNQT:       10000000,
		Deadline:     1440,
		SecretPhrase: "glad suffer red during single glow shut slam hill death lust although"})
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res.TxID)
	}
}

func TestBroadcastTransaction(t *testing.T) {
	res1, err := w.SendMoney(&SendMoneyRequest{
		Recipient:    6418289488649374107,
		AmountNQT:    1,
		FeeNQT:       10000000,
		Deadline:     1440,
		SecretPhrase: "glad suffer red during single glow shut slam hill death lust although",
		Broadcast:    false})
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res1.TxID)
		assert.False(t, res1.Broadcasted)
	}
	res2, err := w.BroadcastTransaction(&BroadcastTransactionRequest{
		TransactionBytes: res1.TransactionBytes,
	})
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res2.TxID)
	}
}

func TestSendMoneyMulti(t *testing.T) {
	res, err := w.SendMoneyMulti(&SendMoneyMultiRequest{
		Recipients:   "12441003299556495598:100000000;11253871103436815155:20000000",
		FeeNQT:       10000000,
		Deadline:     1440,
		SecretPhrase: "glad suffer red during single glow shut slam hill death lust although"})
	if assert.Nil(t, err) {
		assert.NotEmpty(t, res.TxID)
	}
}

func TestGetAccountTransactions(t *testing.T) {
	res, err := w.GetAccountTransactions(&GetAccountTransactionsRequest{
		Account:   5658931570366906527,
		Type:      1,
		Subtype:   0,
		Timestamp: 1})
	if !assert.Nil(t, err) {
		return
	}
	assert.NotEmpty(t, res.Transactions)
}

func TestGetAccount(t *testing.T) {
	res, err := rw.GetAccount(&GetAccountRequest{Account: 12753605638793301951})
	if !assert.Nil(t, err) {
		return
	}
	assert.NotEmpty(t, res.UnconfirmedBalanceNQT)
	assert.NotEmpty(t, res.GuaranteedBalanceNQT)
	assert.NotEmpty(t, res.EffectiveBalanceNXT)
	assert.NotEmpty(t, res.AccountRS)
	assert.NotEmpty(t, res.Name)
	assert.NotEmpty(t, res.ForgedBalanceNQT)
	assert.NotEmpty(t, res.ForgedBalanceNQT)
	assert.NotEmpty(t, res.BalanceNQT)
	assert.NotEmpty(t, res.PublicKey)
	assert.NotEmpty(t, res.Account)
}

func TestGetTransaction(t *testing.T) {
	res, err := w.GetTransaction(&GetTransactionRequest{Transaction: 7877411804310616845})
	if !assert.Nil(t, err) {
		return
	}
	assert.NotEmpty(t, res.SenderPublicKey)
	assert.NotEmpty(t, res.Signature)
	assert.NotEmpty(t, res.FeeNQT)
	assert.NotEmpty(t, res.Confirmations)
	assert.NotEmpty(t, res.FullHash)
	assert.NotEmpty(t, res.Version)
	assert.NotEmpty(t, res.EcBlockID)
	assert.NotEmpty(t, res.SignatureHash)
	if assert.NotEmpty(t, res.Attachment) {
		assert.NotEmpty(t, res.Attachment.Recipients)
		assert.NotEmpty(t, res.Attachment.VersionMultiOutCreation)
	}
	assert.NotEmpty(t, res.SenderRS)
	assert.NotEmpty(t, res.Subtype)
	assert.NotEmpty(t, res.AmountNQT)
	assert.NotEmpty(t, res.Sender)
	assert.NotEmpty(t, res.EcBlockHeight)
	assert.NotEmpty(t, res.Block)
	assert.NotEmpty(t, res.BlockTimestamp)
	assert.NotEmpty(t, res.Deadline)
	assert.NotEmpty(t, res.Transaction)
	assert.NotEmpty(t, res.Timestamp)
	assert.NotEmpty(t, res.Height)
}
