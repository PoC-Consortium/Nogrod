// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package wallet

import (
	. "config"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spebern/globa"
	"io/ioutil"
	. "logger"
	"net/http"
	"strconv"
	"time"
	"util"

	"go.uber.org/zap"
)

type Wallet interface {
	Stop()
	GetMiningInfo() (*MiningInfo, error)
	GetConstantsInfo() (*ConstantsInfo, error)
	GetBlockInfo(uint64) (*BlockInfo, error)
	GetRewardRecipientInfo(uint64) (*RewardRecipientInfo, error)
	SubmitNonce(uint64, uint64) (uint64, error)
	SendPayment(uint64, int64) (uint64, error)
	GetBalance(uint64) (int64, error)
	GetAccountInfo(uint64) (*AccountInfo, error)
	WonBlock(uint64, uint64) (bool, *BlockInfo, error)
	IsPoolRewardRecipient(accountID uint64) (bool, error)
	GetGenerationTime(height uint64) (int32, error)
	GetIncomingMsgsSince(date time.Time, height uint64) (map[uint64]string, error)
}

type BrsWallet struct {
	stopHandlingUrls chan bool
	secretPhrase     string
	client           *http.Client
	lb               globa.LoadBalancer
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

type ConstantsInfo struct {
	GenesisBlockID uint64 `json:"genesisBlockId,string"`
	WithErrorReponse
}

type RewardRecipientInfo struct {
	RewardRecipientID uint64 `json:"rewardRecipient,string"`
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

type GetBalanceResponse struct {
	GuaranteedBalanceNQT int64 `json:"guaranteedBalanceNQT,string"`
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

func NewBrsWallet(walletUrls []string, secretPhrase string) *BrsWallet {
	wallet := BrsWallet{
		secretPhrase:     secretPhrase,
		client:           &http.Client{},
		stopHandlingUrls: make(chan bool),
		lb:               globa.NewLoadBalancer(walletUrls, 30, 10*time.Second, 0.2)}

	go wallet.handleUrls()
	return &wallet
}

func (wallet *BrsWallet) Stop() {
	wallet.stopHandlingUrls <- true
}

func (wallet *BrsWallet) handleUrls() {
	t := time.NewTicker(4 * time.Minute)
	for {
		select {
		case <-t.C:
			wallet.lb.Recover()
		case <-wallet.stopHandlingUrls:
			return
		}
	}
}

func (wallet *BrsWallet) request(method string, queryParams map[string]string) ([]byte, error) {
	walletUrl, err := wallet.lb.GetLeastBusyURL()
	if err != nil {
		Logger.Error("no remaining working wallets... recovering")
		wallet.lb.Recover()
		return []byte{}, err
	}

	startTime, err := wallet.lb.IncLoad(walletUrl)
	if err != nil {
		Logger.Error("timeout on walleturl", zap.String("walletUrl", walletUrl))
		return []byte{}, err
	}

	defer wallet.lb.Done(walletUrl, startTime)

	req, err := http.NewRequest(method, walletUrl+"/burst", nil)
	req.Close = true

	if err != nil {
		Logger.Error("creating request to wallet failed", zap.Error(err))
		return []byte{}, err
	}

	q := req.URL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := wallet.client.Do(req)
	if err != nil {
		Logger.Error("request to wallet failed, removing wallet", zap.Error(err),
			zap.String("walletUrl", walletUrl))
		wallet.lb.Remove(walletUrl)
		return []byte{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Logger.Error("reading response from wallet failed", zap.Error(err),
			zap.String("walletUrl", walletUrl))
		return []byte{}, err
	}

	err = resp.Body.Close()
	if err != nil {
		Logger.Error("closing response body failed", zap.Error(err))
		return []byte{}, err
	}

	return body, nil
}

func (wallet *BrsWallet) GetMiningInfo() (*MiningInfo, error) {
	requestParams := map[string]string{"requestType": "getMiningInfo"}
	jsonBytes, err := wallet.request("GET", requestParams)
	res := MiningInfo{}

	if err != nil {
		Logger.Error("getting mining info failed", zap.Error(err))
		return nil, err
	}

	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		Logger.Error("unpacking miningInfo json failed", zap.Error(err))
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	return &res, nil
}

func (wallet *BrsWallet) GetConstantsInfo() (*ConstantsInfo, error) {
	requestParams := map[string]string{"requestType": "getConstants"}
	jsonBytes, err := wallet.request("GET", requestParams)
	res := ConstantsInfo{}

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		Logger.Error("unpacking constantsInfo json failed", zap.Error(err))
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	return &res, nil
}

func (wallet *BrsWallet) GetBlockInfo(height uint64) (*BlockInfo, error) {
	requestParams := map[string]string{
		"requestType": "getBlock",
		"height":      strconv.FormatUint(height, 10)}
	jsonBytes, err := wallet.request("GET", requestParams)
	res := BlockInfo{}

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		Logger.Error("unpacking blockInfo json failed", zap.Error(err))
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	return &res, nil
}

func (wallet *BrsWallet) GetRewardRecipientInfo(accountID uint64) (*RewardRecipientInfo, error) {
	requestParams := map[string]string{
		"requestType": "getRewardRecipient",
		"account":     strconv.FormatUint(accountID, 10)}
	jsonBytes, err := wallet.request("POST", requestParams)
	res := RewardRecipientInfo{}

	if err != nil {
		Logger.Error("requesting rewardRecipientInfo failed", zap.Error(err))
		return nil, err
	}

	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		Logger.Error("unpacking rewardRecipientInfo json failed", zap.Error(err))
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	return &res, nil
}

func (wallet *BrsWallet) SubmitNonce(nonce uint64, accountID uint64) (uint64, error) {
	requestParams := map[string]string{
		"requestType":  "submitNonce",
		"nonce":        strconv.FormatUint(nonce, 10),
		"accountId":    strconv.FormatUint(accountID, 10),
		"secretPhrase": wallet.secretPhrase}
	jsonBytes, err := wallet.request("POST", requestParams)

	if err != nil {
		return 0, err
	}

	var nonceInfoResp NonceInfoResponse
	err = json.Unmarshal(jsonBytes, &nonceInfoResp)
	if err != nil {
		Logger.Error("unpacking nonceInfo json failed", zap.Error(err))
		return 0, err
	}

	if nonceInfoResp.Result != "success" {
		Logger.Error("submitting nonce failed:", zap.String("result", nonceInfoResp.Result))
		return 0, errors.New(nonceInfoResp.Result)
	}

	return nonceInfoResp.Deadline, nil
}

func (wallet *BrsWallet) SendPayment(accountID uint64, amount int64) (uint64, error) {
	requestParams := map[string]string{
		"requestType":  "sendMoney",
		"recipient":    strconv.FormatUint(accountID, 10),
		"deadline":     "1440",
		"feeNQT":       fmt.Sprint(Cfg.TxFee),
		"amountNQT":    fmt.Sprint(amount),
		"secretPhrase": wallet.secretPhrase}

	jsonBytes, err := wallet.request("POST", requestParams)
	if err != nil {
		return 0, err
	}

	var sendMoneyResp SendMoneyResponse
	err = json.Unmarshal(jsonBytes, &sendMoneyResp)
	if err != nil {
		Logger.Error("unpacking sendMoneyResponse json failed", zap.Error(err))
		return 0, err
	}

	if sendMoneyResp.TxID == 0 {
		Logger.Error("transaction failed", zap.String("errorDescription", sendMoneyResp.ErrorDescription))
		return sendMoneyResp.TxID, errors.New(sendMoneyResp.ErrorDescription)
	}

	return sendMoneyResp.TxID, nil
}

func (wallet *BrsWallet) GetBalance(accountID uint64) (int64, error) {
	requestParams := map[string]string{
		"requestType":           "getGuaranteedBalance",
		"account":               strconv.FormatUint(accountID, 10),
		"numberOfConfirmations": "1"}

	jsonBytes, err := wallet.request("POST", requestParams)
	if err != nil {
		Logger.Error("unpacking balance json failed", zap.Error(err))
		return 0.0, err
	}

	var res GetBalanceResponse
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return 0.0, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return 0.0, errors.New(res.ErrorDescription)
	}

	return res.GuaranteedBalanceNQT, nil
}

func (wallet *BrsWallet) GetAccountInfo(accountID uint64) (*AccountInfo, error) {
	requestParams := map[string]string{
		"requestType": "getAccount",
		"account":     strconv.FormatUint(accountID, 10)}

	jsonBytes, err := wallet.request("POST", requestParams)
	if err != nil {
		Logger.Error("unpacking account info json failed", zap.Error(err))
		return nil, err
	}

	var res AccountInfo
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	return &res, nil
}

func (wallet *BrsWallet) WonBlock(height uint64, nonce uint64) (bool, *BlockInfo, error) {
	// we also need to check the nonce, to be sure that it was submitted from the pool
	blockInfo, err := wallet.GetBlockInfo(height)
	if err != nil {
		return false, blockInfo, err
	}

	rewardRecipientInfo, err := wallet.GetRewardRecipientInfo(blockInfo.GeneratorID)
	if err != nil {
		return false, blockInfo, err
	}

	return rewardRecipientInfo.RewardRecipientID == Cfg.PoolPublicID && blockInfo.Nonce == nonce, blockInfo, nil
}

func (wallet *BrsWallet) IsPoolRewardRecipient(accountID uint64) (bool, error) {
	rewardRecipientInfo, err := wallet.GetRewardRecipientInfo(accountID)
	if err != nil {
		return false, err
	}
	return rewardRecipientInfo.RewardRecipientID == Cfg.PoolPublicID, nil
}

func (wallet *BrsWallet) GetGenerationTime(height uint64) (int32, error) {
	b1, err := wallet.GetBlockInfo(height - 1)
	if err != nil {
		return 0, err
	}

	b2, err := wallet.GetBlockInfo(height)
	if err != nil {
		return 0, err
	}
	return b2.TimeStamp - b1.TimeStamp, nil
}

func (wallet *BrsWallet) GetIncomingMsgsSince(date time.Time, height uint64) (map[uint64]string, error) {
	msgOf := make(map[uint64]string)
	ts := util.DateToTimeStamp(date)
	requestParams := map[string]string{
		"requestType": "getAccountTransactions",
		"account":     strconv.FormatUint(Cfg.PoolPublicID, 10),
		"type":        "1",
		"subtype":     "0",
		"timestamp":   strconv.FormatInt(ts, 10)}

	jsonBytes, err := wallet.request("POST", requestParams)
	if err != nil {
		Logger.Error("getting account transactions failed", zap.Error(err))
		return nil, err
	}

	var res TransactionsInfo
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		Logger.Error("unpacking getAccountTransactions json failed", zap.Error(err))
		return nil, err
	}

	if res.ErrorDescription != "" {
		Logger.Error("wallet returned error", zap.String("errorDescription", res.ErrorDescription),
			zap.Any("reqParams", requestParams))
		return nil, errors.New(res.ErrorDescription)
	}

	for _, transactionInfo := range res.Transactions {
		// already in blockchain?
		// if transactionInfo.Height < height {
		// 	continue
		// }
		if transactionInfo.Sender != Cfg.PoolPublicID {
			msgOf[transactionInfo.Sender] = transactionInfo.Attachment.Msg
		}
	}

	return msgOf, nil
}
