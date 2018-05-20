// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package wallet

import (
	. "config"
	"encoding/json"
	"errors"
	"fmt"
	"goburst/burstmath"
	"io/ioutil"
	. "logger"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type WalletHandler interface {
	GetMiningInfo() (*MiningInfo, error)
	GetBlockInfo(uint64) (*BlockInfo, error)
	SubmitNonce(uint64, uint64) (uint64, error)
	SendPayment(uint64, int64) (uint64, error)
	GetAccountInfo(uint64) (*AccountInfo, error)
	WonBlock(uint64, uint64, uint64) (bool, *BlockInfo, error)
	GetGenerationTime(height uint64) (int32, error)
	GetIncomingMsgsSince(date time.Time) (map[uint64]string, error)
}

type walletHandler struct {
	wallets      []*wallet
	secretPhrase string
}

type wallet struct {
	url     string
	baseURL string
	client  *http.Client
}

type reqResult struct {
	body []byte
	url  *string
	err  error
}

type MiningInfo struct {
	GenerationSignature string `json:"generationSignature"`
	BaseTarget          uint64 `json:"baseTarget,string"`
	Height              uint64 `json:"height,string"`
	WithErrorReponse
}

type BlockInfo struct {
	GeneratorID uint64 `json:"generator,string"`
	BlockReward int64  `json:"blockReward,string"`
	TotalFeeNQT int64  `json:"totalFeeNQT,string"`
	BaseTarget  uint64 `json:"baseTarget,string"`
	Nonce       uint64 `json:"nonce,string"`
	Height      uint64 `json:"height"`
	TimeStamp   int32  `json:"timestamp"`
	WithErrorReponse
}

type NonceInfoResponse struct {
	Deadline uint64 `json:"deadline"`
	Result   string `json:"result"`
	WithErrorReponse
}

type SendMoneyResponse struct {
	TxID uint64 `json:"transaction,string"`
	WithErrorReponse
}

type AccountInfo struct {
	Name string `json:"name"`
	WithErrorReponse
}

type TransactionsInfo struct {
	Transactions []TransactionInfo `json:"transactions"`
	WithErrorReponse
}

type TransactionInfo struct {
	Sender     uint64         `json:"sender,string"`
	Attachment AttachmentInfo `json:"attachment"`
	Height     uint64         `json:"height"`
}

type AttachmentInfo struct {
	Msg string `json:"message"`
}

type WithErrorReponse struct {
	ErrorDescription string `json:"errorDescription,omitempty"`
}

func NewWalletHandler(walletURLS []string, secretPhrase string, timeout time.Duration) WalletHandler {
	wallets := make([]*wallet, len(walletURLS))
	for i, url := range walletURLS {
		wallets[i] = newWallet(url, timeout)
	}
	return &walletHandler{
		wallets:      wallets,
		secretPhrase: secretPhrase}
}

func newWallet(url string, timeout time.Duration) *wallet {
	return &wallet{
		url:     url,
		baseURL: url + "/burst",
		client:  &http.Client{Timeout: timeout}}
}

func (w *wallet) err(err error) *reqResult {
	return &reqResult{
		body: nil,
		err:  err,
		url:  &w.url}
}

func (w *wallet) request(method string, params map[string]string) *reqResult {
	req, err := http.NewRequest(method, w.baseURL, nil)
	if err != nil {
		return w.err(err)
	}

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := w.client.Do(req)
	if err != nil {
		return w.err(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return w.err(err)
	}
	resp.Body.Close()

	return &reqResult{
		body: body,
		url:  &w.url}
}

func (wh *walletHandler) requestAll(method string, params map[string]string) ([]*reqResult, error) {
	resultsC := make(chan *reqResult)
	for _, w := range wh.wallets {
		go func(w *wallet) {
			resultsC <- w.request(method, params)
		}(w)
	}

	var results []*reqResult
	var responded int
	for res := range resultsC {
		responded++
		if res.err == nil {
			results = append(results, res)
		}
		if responded == len(wh.wallets) {
			break
		}
	}

	if len(results) == 0 {
		return nil, errors.New("no wallet responded successfull within timeout")
	}
	return results, nil
}

func (wh *walletHandler) GetMiningInfo() (*MiningInfo, error) {
	results, err := wh.requestAll("GET", map[string]string{"requestType": "getMiningInfo"})
	if err != nil {
		return nil, err
	}

	miningInfo := MiningInfo{}
	for _, res := range results {
		miningInfoTmp := MiningInfo{}
		err = json.Unmarshal(res.body, &miningInfoTmp)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}

		if miningInfoTmp.ErrorDescription != "" {
			logErrDescription(*res.url, miningInfoTmp.ErrorDescription)
			continue
		}

		if miningInfoTmp.Height > miningInfo.Height {
			miningInfo = miningInfoTmp
		} else if miningInfoTmp.Height < miningInfo.Height {
			Logger.Warn("wallet lacks behind other wallets",
				zap.Uint64("height", miningInfoTmp.Height),
				zap.Uint64("others height", miningInfo.Height))
		}
	}

	if miningInfo.Height == 0 {
		return nil, errors.New("no wallet successfull in getting mining info")
	}
	return &miningInfo, err
}

func (wh *walletHandler) GetBlockInfo(height uint64) (*BlockInfo, error) {
	results, err := wh.requestAll("GET", map[string]string{
		"requestType": "getBlock",
		"height":      strconv.FormatUint(height, 10)})
	if err != nil {
		return nil, err
	}

	var blockInfo BlockInfo
	for _, res := range results {
		err = json.Unmarshal(res.body, &blockInfo)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}

		if blockInfo.ErrorDescription != "" {
			logErrDescription(*res.url, blockInfo.ErrorDescription)
			continue
		}

		return &blockInfo, nil
	}
	return nil, errors.New("no wallet successfull in getting block info")
}

func (wh *walletHandler) SubmitNonce(nonce uint64, accountID uint64) (uint64, error) {
	results, err := wh.requestAll("POST", map[string]string{
		"requestType":  "submitNonce",
		"nonce":        strconv.FormatUint(nonce, 10),
		"accountId":    strconv.FormatUint(accountID, 10),
		"secretPhrase": wh.secretPhrase})
	if err != nil {
		return 0, err
	}

	var nonceInfoResp NonceInfoResponse
	for _, res := range results {
		err = json.Unmarshal(res.body, &nonceInfoResp)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}

		if nonceInfoResp.Result != "success" {
			logErrDescription(*res.url, nonceInfoResp.Result)
			continue
		}

		return nonceInfoResp.Deadline, nil
	}

	return 0, errors.New("no wallet successfull in submitting nonce")
}

func (wh *walletHandler) SendPayment(accountID uint64, amount int64) (uint64, error) {
	params := map[string]string{
		"requestType":  "sendMoney",
		"recipient":    strconv.FormatUint(accountID, 10),
		"deadline":     "1440",
		"feeNQT":       fmt.Sprint(Cfg.TxFee),
		"amountNQT":    fmt.Sprint(amount),
		"secretPhrase": wh.secretPhrase}

	var sendMoneyResp SendMoneyResponse
	for _, w := range wh.wallets {
		res := w.request("POST", params)
		if res.err != nil {
			Logger.Error("send money request failed", zap.String("wallet", *res.url), zap.Error(res.err))
			continue
		}

		err := json.Unmarshal(res.body, &sendMoneyResp)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}
		if sendMoneyResp.TxID == 0 {
			logErrDescription(*res.url, sendMoneyResp.ErrorDescription)
			continue
		}

		return sendMoneyResp.TxID, nil
	}
	return 0, errors.New("no wallet successfull in sending money")
}

func (wh *walletHandler) GetAccountInfo(accountID uint64) (*AccountInfo, error) {
	results, err := wh.requestAll("POST", map[string]string{
		"requestType": "getAccount",
		"account":     strconv.FormatUint(accountID, 10)})
	if err != nil {
		return nil, err
	}

	var accountInfo AccountInfo
	for _, res := range results {
		err = json.Unmarshal(res.body, &accountInfo)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}

		if accountInfo.ErrorDescription != "" {
			logErrDescription(*res.url, accountInfo.ErrorDescription)
			continue
		}
		return &accountInfo, nil
	}
	return nil, errors.New("no wallet successfull in gettting account info")
}

func (wh *walletHandler) WonBlock(height uint64, minerID, nonce uint64) (bool, *BlockInfo, error) {
	// we also need to check the nonce, to be sure that it was submitted from the pool
	blockInfo, err := wh.GetBlockInfo(height)
	if err != nil {
		return false, blockInfo, err
	}

	Logger.Info("checking if block was one",
		zap.Uint64("generator", blockInfo.GeneratorID),
		zap.Uint64("nonce", blockInfo.Nonce),
		zap.Uint64("expected generator", minerID),
		zap.Uint64("expected nonce", nonce),
		zap.Bool("was won", blockInfo.GeneratorID == minerID && blockInfo.Nonce == nonce),
		zap.Uint64("height", height))

	return blockInfo.GeneratorID == minerID && blockInfo.Nonce == nonce, blockInfo, nil
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
	return b2.TimeStamp - b1.TimeStamp, nil
}

func (wh *walletHandler) GetIncomingMsgsSince(date time.Time) (map[uint64]string, error) {
	msgOf := make(map[uint64]string)
	ts := burstmath.DateToTimeStamp(date)

	results, err := wh.requestAll("POST", map[string]string{
		"requestType": "getAccountTransactions",
		"account":     strconv.FormatUint(Cfg.PoolPublicID, 10),
		"type":        "1",
		"subtype":     "0",
		"timestamp":   strconv.FormatInt(ts, 10)})

	var txsInfo TransactionsInfo
	for _, res := range results {
		err = json.Unmarshal(res.body, &txsInfo)
		if err != nil {
			logJSONUnpackingErr(*res.url, err, res.body)
			continue
		}

		if txsInfo.ErrorDescription != "" {
			logErrDescription(*res.url, txsInfo.ErrorDescription)
			continue
		}

		for _, txInfo := range txsInfo.Transactions {
			if txInfo.Sender != Cfg.PoolPublicID {
				msgOf[txInfo.Sender] = txInfo.Attachment.Msg
			}
		}
		return msgOf, nil
	}
	return nil, errors.New("no wallet successfull in getting incoming msgs")
}

func logJSONUnpackingErr(wallet string, err error, msg []byte) {
	Logger.Error("unpacking json failed", zap.String("wallet", wallet), zap.Error(err),
		zap.String("msg", string(msg)))
}

func logErrDescription(wallet string, errDescription string) {
	Logger.Error("wallet request returned error", zap.String("wallet", wallet),
		zap.String("errorDescription", errDescription))
}
