// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package wallethandler

import (
	. "config"
	"errors"
	"fmt"
	"goburst/burstmath"
	"goburst/wallet"
	. "logger"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type WalletHandler interface {
	GetMiningInfo() (*wallet.GetMiningInfoReply, error)
	GetBlockInfo(uint64) (*wallet.GetBlockReply, error)
	SubmitNonce(uint64, uint64, uint64) error
	SendPayment(uint64, int64) (uint64, error)
	SendPayments(map[uint64]int64) (uint64, error)
	GetAccountInfo(uint64) (*wallet.GetAccountReply, error)
	WonBlock(uint64, uint64, uint64) (bool, *wallet.GetBlockReply, error)
	GetGenerationTime(height uint64) (int32, error)
	GetIncomingMsgsSince(date time.Time) (map[uint64]string, error)
	GetRewardRecipients() (map[uint64]bool, error)
	GetTransaction(uint64) (*wallet.GetTransactionReply, bool, error)
	CalcOptimalTxFee(uint64) (int64, error)
}

type walletHandler struct {
	wallets      map[string]wallet.Wallet
	secretPhrase string
}

type reqRes struct {
	obj interface{}
	url string
}

func NewWalletHandler(walletURLS []string, secretPhrase string, timeout time.Duration, trustAll bool) WalletHandler {
	wallets := make(map[string]wallet.Wallet)
	for _, u := range walletURLS {
		wallets[u] = wallet.NewWallet(u, timeout, trustAll)
	}
	return &walletHandler{
		wallets:      wallets,
		secretPhrase: secretPhrase}
}

func (wh *walletHandler) reqAll(reqF func(wallet.Wallet) (interface{}, error)) ([]reqRes, error) {
	var results []reqRes
	var mu sync.Mutex
	var wg sync.WaitGroup
	for u, w := range wh.wallets {
		wg.Add(1)
		go func(u string, w wallet.Wallet) {
			defer wg.Done()
			obj, err := reqF(w)
			if err != nil {
				Logger.Error("request to wallet", zap.Error(err))
				return
			}
			mu.Lock()
			results = append(results, reqRes{obj: obj, url: u})
			mu.Unlock()
		}(u, w)
	}
	wg.Wait()
	if len(results) == 0 {
		return nil, errors.New("no wallet sucessfull")
	}
	return results, nil
}

func (wh *walletHandler) reqRandom(reqF func(wallet.Wallet) (interface{}, error)) (interface{}, error) {
	// random hash traversal
	for _, w := range wh.wallets {
		obj, err := reqF(w)
		if err == nil {
			return obj, nil
		}
		Logger.Error("request to wallet", zap.Error(err))
	}
	return nil, errors.New("no wallet successfull")
}

func (wh *walletHandler) GetMiningInfo() (*wallet.GetMiningInfoReply, error) {
	results, err := wh.reqAll(func(w wallet.Wallet) (interface{}, error) {
		return w.GetMiningInfo()
	})
	if err != nil {
		return nil, fmt.Errorf("getting mining info: %v", err)
	}
	miningInfo := wallet.GetMiningInfoReply{}
	for _, res := range results {
		miningInfoTmp := res.obj.(*wallet.GetMiningInfoReply)

		if miningInfoTmp.Height > miningInfo.Height {
			miningInfo = *miningInfoTmp
		} else if miningInfoTmp.Height < miningInfo.Height {
			Logger.Warn("wallet lacks behind other wallets",
				zap.String("url", res.url),
				zap.Uint64("height", miningInfoTmp.Height),
				zap.Uint64("others height", miningInfo.Height))
		}
	}
	return &miningInfo, nil
}

func (wh *walletHandler) GetBlockInfo(height uint64) (*wallet.GetBlockReply, error) {
	res, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.GetBlock(&wallet.GetBlockRequest{Height: height})
	})
	if err != nil {
		return nil, fmt.Errorf("get block info: %v", err)
	}
	return res.(*wallet.GetBlockReply), nil
}

func (wh *walletHandler) SubmitNonce(nonce uint64, accountID uint64, deadline uint64) error {
	_, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		res, err := w.SubmitNonce(&wallet.SubmitNonceRequest{
			AccountID:    accountID,
			Nonce:        nonce,
			SecretPhrase: wh.secretPhrase})
		if err != nil {
			return nil, err
		}
		walletDeadline := res.Deadline
		if walletDeadline != deadline {
			return nil, fmt.Errorf(
				"pool deadline %d doesn't match wallet deadline %d",
				deadline, walletDeadline)
		}
		return res, nil
	})
	return err
}

func (wh *walletHandler) SendPayment(recipient uint64, amount int64) (uint64, error) {
	obj, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.SendMoney(&wallet.SendMoneyRequest{
			Recipient:    recipient,
			Deadline:     1440,
			FeeNQT:       Cfg.PoolTxFee,
			AmountNQT:    amount,
			SecretPhrase: wh.secretPhrase})
	})
	if err != nil {
		return 0, err
	}
	return obj.(*wallet.SendMoneyReply).TxID, nil
}

func (wh *walletHandler) SendPayments(idToAmount map[uint64]int64) (uint64, error) {
	recipients, err := wallet.EncodeRecipients(idToAmount)
	if err != nil {
		return 0, err
	}
	obj, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.SendMoneyMulti(&wallet.SendMoneyMultiRequest{
			Recipients:   recipients,
			Deadline:     1440,
			FeeNQT:       Cfg.PoolTxFee,
			SecretPhrase: wh.secretPhrase})
	})
	if err != nil {
		return 0, err
	}
	return obj.(*wallet.SendMoneyMultiReply).TxID, nil
}

func (wh *walletHandler) GetAccountInfo(accountID uint64) (*wallet.GetAccountReply, error) {
	obj, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.GetAccount(&wallet.GetAccountRequest{
			Account: accountID})
	})
	if err != nil {
		return nil, err
	}
	return obj.(*wallet.GetAccountReply), nil
}

func (wh *walletHandler) WonBlock(height, minerID, nonce uint64) (bool, *wallet.GetBlockReply, error) {
	// we also need to check the nonce, to be sure that it was submitted from the pool
	blockInfo, err := wh.GetBlockInfo(height)
	if err != nil {
		return false, blockInfo, err
	}

	Logger.Info("checking if block was won",
		zap.Uint64("generator", blockInfo.Generator),
		zap.Uint64("nonce", blockInfo.Nonce),
		zap.Uint64("expected generator", minerID),
		zap.Uint64("expected nonce", nonce),
		zap.Bool("was won", blockInfo.Generator == minerID && blockInfo.Nonce == nonce),
		zap.Uint64("height", height))

	return blockInfo.Generator == minerID && blockInfo.Nonce == nonce, blockInfo, nil
}

func (wh *walletHandler) GetGenerationTime(height uint64) (int32, error) {
	b1, err := wh.GetBlockInfo(height - 1)
	if err != nil {
		return 0, err
	}

	b2, err := wh.GetBlockInfo(height)
	if err != nil {
		return 0, err
	}
	return b2.Timestamp - b1.Timestamp, nil
}

func (wh *walletHandler) GetIncomingMsgsSince(date time.Time) (map[uint64]string, error) {
	obj, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.GetAccountTransactions(&wallet.GetAccountTransactionsRequest{
			Account:   Cfg.PoolPublicID,
			Type:      1,
			Subtype:   0,
			Timestamp: burstmath.DateToTimeStamp(date)})
	})
	if err != nil {
		return nil, err
	}
	msgOf := make(map[uint64]string)

	for _, txInfo := range obj.(*wallet.GetAccountTransactionsReply).Transactions {
		if txInfo.Sender != Cfg.PoolPublicID {
			msgOf[txInfo.Sender] = txInfo.Attachment.Message
		}
	}
	return msgOf, nil
}

func (wh *walletHandler) GetRewardRecipients() (map[uint64]bool, error) {
	// TODO: we should always get the newest reward recipients, that means
	// the reward recipients from the wallet with the longest block chain
	res, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		return w.GetAccountsWithRewardRecipient(&wallet.GetAccountsWithRewardRecipientRequest{
			AccountID: Cfg.PoolPublicID})
	})
	if err != nil {
		return nil, err
	}
	recips := make(map[uint64]bool)
	for _, r := range res.(*wallet.GetAccountsWithRewardRecipientReply).Recipients {
		recips[uint64(r)] = true
	}
	return recips, nil
}

func (wh *walletHandler) GetTransaction(txID uint64) (*wallet.GetTransactionReply, bool, error) {
	var querySuccessful bool
	var mu sync.Mutex
	res, err := wh.reqRandom(func(w wallet.Wallet) (interface{}, error) {
		obj, err := w.GetTransaction(&wallet.GetTransactionRequest{Transaction: txID})
		// TODO: suffix is not the most stable way...
		if err == nil || strings.HasSuffix(err.Error(), "Unknown transaction") {
			mu.Lock()
			querySuccessful = true
			mu.Unlock()
		}
		return obj, err
	})
	if err != nil {
		return nil, querySuccessful, err
	}
	return res.(*wallet.GetTransactionReply), querySuccessful, nil
}

func (wh *walletHandler) CalcOptimalTxFee(height uint64) (int64, error) {
	var txCount int64
	for h := height - 11; h < height-1; h++ {
		blockInfo, err := wh.GetBlockInfo(h)
		if err != nil {
			return 0, err
		}
		txCount += int64(blockInfo.NumberOfTransactions)
	}
	return 735000 * (txCount/10 + 1), nil
}
