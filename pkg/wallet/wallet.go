package wallet

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/valyala/fasthttp"
)

const MaxMultiRecipients = 64

type requestTypeField struct {
	RequestType string `url:"requestType"`
}

type failable interface {
	getError() string
}

type errorDescriptionField struct {
	ErrorDescription string `json:"errorDescription,omitempty"`
}

func (ef errorDescriptionField) getError() string {
	return ef.ErrorDescription
}

type transactionData struct {
	SignatureHash            string `json:"signatureHash"`
	UnsignedTransactionBytes string `json:"unsignedTransactionBytes"`
	TransactionJSON          struct {
		SenderPublicKey string `json:"senderPublicKey"`
		Signature       string `json:"signature"`
		FeeNQT          string `json:"feeNQT"`
		Type            int    `json:"type"`
		FullHash        string `json:"fullHash"`
		Version         int    `json:"version"`
		EcBlockID       uint64 `json:"ecBlockId,string"`
		SignatureHash   string `json:"signatureHash"`
		SenderRS        string `json:"senderRS"`
		Subtype         int    `json:"subtype"`
		AmountNQT       int64  `json:"amountNQT,string"`
		Sender          uint64 `json:"sender,string"`
		RecipientRS     string `json:"recipientRS"`
		Recipient       uint64 `json:"recipient,string"`
		EcBlockHeight   uint64 `json:"ecBlockHeight"`
		Deadline        int    `json:"deadline"`
		Transaction     uint64 `json:"transaction,string"`
		Timestamp       int    `json:"timestamp"`
		Height          uint64 `json:"height"`
	} `json:"transactionJSON"`
	Broadcasted      bool   `json:"broadcasted"`
	TransactionBytes string `json:"transactionBytes"`
	FullHash         string `json:"fullHash"`
}

type GetMiningInfoRequest struct {
	requestTypeField
	res GetMiningInfoReply `url:"-"`
}

type GetMiningInfoReply struct {
	GenerationSignature string `json:"generationSignature"`
	BaseTarget          uint64 `json:"baseTarget,string"`
	Height              uint64 `json:"height,string"`
	errorDescriptionField
}

type SubmitNonceRequest struct {
	requestTypeField
	AccountID    uint64 `url:"accountId"`
	Nonce        uint64 `url:"nonce"`
	SecretPhrase string `url:"secretPhrase"`
	res          SubmitNonceReply
}

type SubmitNonceReply struct {
	Deadline uint64 `json:"deadline"`
	Result   string `json:"result"`
	errorDescriptionField
}

type GetBlockRequest struct {
	requestTypeField
	Block               uint64 `url:"block,omitempty"`
	Height              uint64 `url:"height,omitempty"`
	Timestamp           int64  `url:"timestamp,omitempty"`
	IncludeTransactions bool   `url:"includeTransactions,omitempty"`
	res                 GetBlockReply
}

type GetBlockReply struct {
	PreviousBlockHash    string      `json:"previousBlockHash"`
	PayloadLength        int         `json:"payloadLength"`
	TotalAmountNQT       int64       `json:"totalAmountNQT,string"`
	GenerationSignature  string      `json:"generationSignature"`
	Generator            uint64      `json:"generator,string"`
	GeneratorPublicKey   string      `json:"generatorPublicKey"`
	BaseTarget           uint64      `json:"baseTarget,string"`
	PayloadHash          string      `json:"payloadHash"`
	GeneratorRS          string      `json:"generatorRS"`
	BlockReward          int64       `json:"blockReward,string"`
	ScoopNum             uint32      `json:"scoopNum"`
	NumberOfTransactions int         `json:"numberOfTransactions"`
	BlockSignature       string      `json:"blockSignature"`
	Transactions         []Uint64Str `json:"transactions"`
	Nonce                uint64      `json:"nonce,string"`
	Version              int         `json:"version"`
	TotalFeeNQT          int64       `json:"totalFeeNQT,string"`
	PreviousBlock        uint64      `json:"previousBlock,string"`
	Block                uint64      `json:"block,string"`
	NextBlock            uint64      `json:"nextBlock,string"`
	Height               uint64      `json:"height"`
	Timestamp            int32       `json:"timestamp"`
	errorDescriptionField
}

type GetAccountsWithRewardRecipientRequest struct {
	requestTypeField
	AccountID uint64 `url:"account"`
	res       GetAccountsWithRewardRecipientReply
}

type GetAccountsWithRewardRecipientReply struct {
	Recipients []Uint64Str `json:"accounts"`
	errorDescriptionField
}

type SendMoneyRequest struct {
	requestTypeField
	Recipient                     uint64 `url:"recipient,string"`
	AmountNQT                     int64  `url:"amountNQT,string"`
	FeeNQT                        int64  `url:"feeNQT,string"`
	Deadline                      uint   `url:"deadline"`
	ReferencedTransactionFullHash string `url:"referencedTransactionFullHash,omitempty"`
	Broadcast                     bool   `url:"broadcast"`
	SecretPhrase                  string `url:"secretPhrase"`
	res                           SendMoneyReply
}

type BroadcastTransactionRequest struct {
	requestTypeField
	TransactionBytes string `url:"transactionBytes,omitempty"`
	TransactionJSON  string `url:"transactionJSON,omitempty"`
	res              BroadcastTransactionReply
}

type BroadcastTransactionReply struct {
	FullHash string `json:"fullHash"`
	TxID     uint64 `json:"transaction,string"`
	errorDescriptionField
}

type SendMoneyReply struct {
	TxID uint64 `json:"transaction,string"`
	transactionData
	errorDescriptionField
}

type SendMoneyMultiRequest struct {
	requestTypeField
	Recipients                    string `url:"recipients"`
	FeeNQT                        int64  `url:"feeNQT,string"`
	Deadline                      uint   `url:"deadline"`
	ReferencedTransactionFullHash string `url:"referencedTransactionFullHash,omitempty"`
	Broadcast                     bool   `url:"broadcast"`
	SecretPhrase                  string `url:"secretPhrase"`
	res                           SendMoneyMultiReply
}

type SendMoneyMultiReply struct {
	TxID uint64 `json:"transaction,string"`
	transactionData
	errorDescriptionField
}

type GetAccountTransactionsRequest struct {
	requestTypeField
	Account               uint64 `url:"account,string"`
	Timestamp             int64  `url:"timestamp,omitempty"`
	Type                  int    `url:"type,omitempty"`
	Subtype               int    `url:"type,omitempty"`
	FirstIndex            int    `url:"firstIndex,omitempty"`
	LastIndex             int    `url:"lastIndex,omitempty"`
	NumberOfConfirmations int    `url:"numberOfConfirmations,omitempty"`
	res                   GetAccountTransactionsReply
}

type GetAccountTransactionsReply struct {
	Transactions []struct {
		SenderPublicKey string `json:"senderPublicKey"`
		Signature       string `json:"signature"`
		FeeNQT          int64  `json:"feeNQT,string"`
		Type            int    `json:"type"`
		Confirmations   int    `json:"confirmations"`
		FullHash        string `json:"fullHash"`
		Version         int    `json:"version"`
		EcBlockID       uint64 `json:"ecBlockId,string"`
		SignatureHash   string `json:"signatureHash"`
		Attachment      struct {
			VersionMessage int    `json:"version.Message"`
			MessageIsText  bool   `json:"messageIsText"`
			Message        string `json:"message"`
		} `json:"attachment"`
		SenderRS       string `json:"senderRS"`
		Subtype        int    `json:"subtype"`
		AmountNQT      int64  `json:"amountNQT,string"`
		Sender         uint64 `json:"sender,string"`
		RecipientRS    string `json:"recipientRS"`
		Recipient      uint64 `json:"recipient,string"`
		EcBlockHeight  uint64 `json:"ecBlockHeight"`
		Block          string `json:"block"`
		BlockTimestamp int64  `json:"blockTimestamp"`
		Deadline       uint64 `json:"deadline"`
		Transaction    uint64 `json:"transaction,string"`
		Timestamp      int64  `json:"timestamp"`
		Height         uint64 `json:"height"`
	} `json:"transactions"`
	errorDescriptionField
}

type GetAccountRequest struct {
	requestTypeField
	Account uint64 `url:"account"`
	res     GetAccountReply
}

type GetAccountReply struct {
	UnconfirmedBalanceNQT int64  `json:"unconfirmedBalanceNQT,string"`
	GuaranteedBalanceNQT  int64  `json:"guaranteedBalanceNQT,string"`
	EffectiveBalanceNXT   int64  `json:"effectiveBalanceNXT,string"`
	AccountRS             string `json:"accountRS"`
	Name                  string `json:"name"`
	ForgedBalanceNQT      int64  `json:"forgedBalanceNQT,string"`
	BalanceNQT            int64  `json:"balanceNQT,string"`
	PublicKey             string `json:"publicKey"`
	Account               uint64 `json:"account,string"`
	errorDescriptionField
}

type GetTransactionRequest struct {
	requestTypeField
	Transaction uint64 `url:"transaction"`
	FullHash    string `url:"fullHash"`
	res         GetTransactionReply
}

type GetTransactionReply struct {
	SenderPublicKey string `json:"senderPublicKey"`
	Signature       string `json:"signature"`
	FeeNQT          int64  `json:"feeNQT,string"`
	Type            int    `json:"type"`
	Confirmations   int    `json:"confirmations"`
	FullHash        string `json:"fullHash"`
	Version         int    `json:"version"`
	EcBlockID       uint64 `json:"ecBlockId,string"`
	SignatureHash   string `json:"signatureHash"`
	// TODO: recipients should be decoded into something like
	// struct { AccountID uint64, Amount int64 }
	Attachment struct {
		Recipients              [][]string `json:"recipients"`
		VersionMultiOutCreation int        `json:"version.MultiOutCreation"`
	} `json:"attachment"`
	SenderRS       string `json:"senderRS"`
	Subtype        int    `json:"subtype"`
	AmountNQT      int64  `json:"amountNQT,string"`
	Sender         uint64 `json:"sender,string"`
	EcBlockHeight  uint64 `json:"ecBlockHeight"`
	Block          uint64 `json:"block,string"`
	BlockTimestamp int64  `json:"blockTimestamp"`
	Deadline       int    `json:"deadline"`
	Transaction    uint64 `json:"transaction,string"`
	Timestamp      int64  `json:"timestamp"`
	Height         uint64 `json:"height"`
	errorDescriptionField
}

type Wallet interface {
	BroadcastTransaction(*BroadcastTransactionRequest) (*BroadcastTransactionReply, error)
	// BuyAlias() (*BuyAliasReply, error)
	// CalculateFullHash() (*CalculateFullHashReply, error)
	// CancelAskOrder() (*CancelAskOrderReply, error)
	// CancelBidOrder() (*CancelBidOrderReply, error)
	// CreateATProgram() (*CreateATProgramReply, error)
	// DecodeHallmark() (*DecodeHallmarkReply, error)
	// DecodeToken() (*DecodeTokenReply, error)
	// DecryptFrom() (*DecryptFromReply, error)
	// DgsDelisting() (*DgsDelistingReply, error)
	// DgsDelivery() (*DgsDeliveryReply, error)
	// DgsFeedback() (*DgsFeedbackReply, error)
	// DgsListing() (*DgsListingReply, error)
	// DgsPriceChange() (*DgsPriceChangeReply, error)
	// DgsPurchase() (*DgsPurchaseReply, error)
	// DgsQuantityChange() (*DgsQuantityChangeReply, error)
	// DgsRefund() (*DgsRefundReply, error)
	// EncryptTo() (*EncryptToReply, error)
	// EscrowSign() (*EscrowSignReply, error)
	// GenerateToken() (*GenerateTokenReply, error)
	// GetAT() (*GetATReply, error)
	// GetATDetails() (*GetATDetailsReply, error)
	// GetATIds() (*GetATIdsReply, error)
	// GetATLong() (*GetATLongReply, error)
	GetAccount(*GetAccountRequest) (*GetAccountReply, error)
	// GetAccountATs() (*GetAccountATsReply, error)
	// GetAccountBlockIds() (*GetAccountBlockIdsReply, error)
	// GetAccountBlocks() (*GetAccountBlocksReply, error)
	// GetAccountCurrentAskOrderIds() (*GetAccountCurrentAskOrderIdsReply, error)
	// GetAccountCurrentAskOrders() (*GetAccountCurrentAskOrdersReply, error)
	// GetAccountCurrentBidOrderIds() (*GetAccountCurrentBidOrderIdsReply, error)
	// GetAccountCurrentBidOrders() (*GetAccountCurrentBidOrdersReply, error)
	// GetAccountEscrowTransactions() (*GetAccountEscrowTransactionsReply, error)
	// GetAccountId() (*GetAccountIdReply, error)
	// GetAccountLessors() (*GetAccountLessorsReply, error)
	// GetAccountPublicKey() (*GetAccountPublicKeyReply, error)
	// GetAccountSubscriptions() (*GetAccountSubscriptionsReply, error)
	// GetAccountTransactionIds() (*GetAccountTransactionIdsReply, error)
	GetAccountTransactions(*GetAccountTransactionsRequest) (*GetAccountTransactionsReply, error)
	GetAccountsWithRewardRecipient(*GetAccountsWithRewardRecipientRequest) (
		*GetAccountsWithRewardRecipientReply, error)
	// GetAlias() (*GetAliasReply, error)
	// GetAliases() (*GetAliasesReply, error)
	// GetAllAssets() (*GetAllAssetsReply, error)
	// GetAllOpenAskOrders() (*GetAllOpenAskOrdersReply, error)
	// GetAllOpenBidOrders() (*GetAllOpenBidOrdersReply, error)
	// GetAllTrades() (*GetAllTradesReply, error)
	// GetAskOrder() (*GetAskOrderReply, error)
	// GetAskOrderIds() (*GetAskOrderIdsReply, error)
	// GetAskOrders() (*GetAskOrdersReply, error)
	// GetAsset() (*GetAssetReply, error)
	// GetAssetAccounts() (*GetAssetAccountsReply, error)
	// GetAssetIds() (*GetAssetIdsReply, error)
	// GetAssetTransfers() (*GetAssetTransfersReply, error)
	// GetAssets() (*GetAssetsReply, error)
	// GetAssetsByIssuer() (*GetAssetsByIssuerReply, error)
	// GetBalance() (*GetBalanceReply, error)
	// GetBidOrder() (*GetBidOrderReply, error)
	// GetBidOrderIds() (*GetBidOrderIdsReply, error)
	// GetBidOrders() (*GetBidOrdersReply, error)
	GetBlock(*GetBlockRequest) (*GetBlockReply, error)
	// GetBlockId() (*GetBlockIdReply, error)
	// GetBlockchainStatus() (*GetBlockchainStatusReply, error)
	// GetBlocks() (*GetBlocksReply, error)
	// GetConstants() (*GetConstantsReply, error)
	// GetDGSGood() (*GetDGSGoodReply, error)
	// GetDGSGoods() (*GetDGSGoodsReply, error)
	// GetDGSPendingPurchases() (*GetDGSPendingPurchasesReply, error)
	// GetDGSPurchase() (*GetDGSPurchaseReply, error)
	// GetDGSPurchases() (*GetDGSPurchasesReply, error)
	// GetECBlock() (*GetECBlockReply, error)
	// GetEscrowTransaction() (*GetEscrowTransactionReply, error)
	// GetGuaranteedBalance() (*GetGuaranteedBalanceReply, error)
	GetMiningInfo() (*GetMiningInfoReply, error)
	// GetMyInfo() (*GetMyInfoReply, error)
	// GetPeer() (*GetPeerReply, error)
	// GetPeers() (*GetPeersReply, error)
	// GetRewardRecipient() (*GetRewardRecipientReply, error)
	// GetState() (*GetStateReply, error)
	// GetSubscription() (*GetSubscriptionReply, error)
	// GetSubscriptionsToAccount() (*GetSubscriptionsToAccountReply, error)
	// GetTime() (*GetTimeReply, error)
	// GetTrades() (*GetTradesReply, error)
	GetTransaction(*GetTransactionRequest) (*GetTransactionReply, error)
	// GetTransactionBytes() (*GetTransactionBytesReply, error)
	// GetUnconfirmedTransactionIds() (*GetUnconfirmedTransactionIdsReply, error)
	// GetUnconfirmedTransactions() (*GetUnconfirmedTransactionsReply, error)
	// IssueAsset() (*IssueAssetReply, error)
	// LeaseBalance() (*LeaseBalanceReply, error)
	// LongConvert() (*LongConvertReply, error)
	// MarkHost() (*MarkHostReply, error)
	// ParseTransaction() (*ParseTransactionReply, error)
	// PlaceAskOrder() (*PlaceAskOrderReply, error)
	// PlaceBidOrder() (*PlaceBidOrderReply, error)
	// ReadMessage() (*ReadMessageReply, error)
	// RsConvert() (*RsConvertReply, error)
	// SellAlias() (*SellAliasReply, error)
	// SendMessage() (*SendMessageReply, error)
	SendMoney(*SendMoneyRequest) (*SendMoneyReply, error)
	SendMoneyMulti(*SendMoneyMultiRequest) (*SendMoneyMultiReply, error)
	// SendMoneyEscrow() (*SendMoneyEscrowReply, error)
	// SendMoneySubscription() (*SendMoneySubscriptionReply, error)
	// SetAccountInfo() (*SetAccountInfoReply, error)
	// SetAlias() (*SetAliasReply, error)
	// SetRewardRecipient() (*SetRewardRecipientReply, error)
	// SignTransaction() (*SignTransactionReply, error)
	SubmitNonce(*SubmitNonceRequest) (*SubmitNonceReply, error)
	// SubscriptionCancel() (*SubscriptionCancelReply, error)
	// TransferAsset() (*TransferAssetReply, error)
}

type wallet struct {
	client       *fasthttp.Client
	apiURL       string
	secretPhrase string
}

type Uint64Str uint64

func (i Uint64Str) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(i), 10))
}

func (i *Uint64Str) UnmarshalJSON(b []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		value, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		*i = Uint64Str(value)
		return nil
	}

	// Fallback to number
	return json.Unmarshal(b, (*uint64)(i))
}

func NewWallet(url string, timeout time.Duration, trustAll bool) Wallet {
	client := fasthttp.Client{
		ReadTimeout:  timeout,
		WriteTimeout: timeout}
	if trustAll {
		client.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &wallet{
		apiURL: url + "/burst?",
		client: &client}
}

func EncodeRecipients(idToAmount map[uint64]int64) (string, error) {
	if len(idToAmount) == 0 {
		return "", errors.New("no recipients")
	}
	if len(idToAmount) > MaxMultiRecipients {
		panic(fmt.Sprintf("cannot send to more than %d recipients at a time", MaxMultiRecipients))
	}
	recipients := ""
	for accountID, amount := range idToAmount {
		recipients += strconv.FormatUint(accountID, 10) + ":" + fmt.Sprint(amount) + ";"
	}
	return recipients[:len(recipients)-1], nil
}

func (w *wallet) processJSONRequest(method string, queryStruct interface{}, dest failable) error {
	v, err := query.Values(queryStruct)
	if err != nil {
		return err
	}
	u := w.apiURL + v.Encode()

	var body []byte
	var statusCode int
	if method == "GET" {
		statusCode, body, err = w.client.Get(body, u)
	} else {
		statusCode, body, err = w.client.Post(body, u, nil)
	}

	if err != nil {
		return fmt.Errorf("request to %s: %v", u, err)
	}

	if statusCode != fasthttp.StatusOK {
		return fmt.Errorf("wrong status code: %d for url %s", statusCode, u)
	}

	err = json.Unmarshal(body, dest)
	if err != nil {
		return err
	}

	if errDescription := dest.getError(); errDescription != "" {
		return fmt.Errorf("request to %s: %s", u, errDescription)
	}
	return nil
}

func (w *wallet) GetMiningInfo() (*GetMiningInfoReply, error) {
	req := GetMiningInfoRequest{}
	req.RequestType = "getMiningInfo"
	return &req.res, w.processJSONRequest("GET", req, &req.res)
}

func (w *wallet) SubmitNonce(req *SubmitNonceRequest) (*SubmitNonceReply, error) {
	req.RequestType = "submitNonce"
	return &req.res, w.processJSONRequest("POST", req, &req.res)
}

func (w *wallet) GetBlock(req *GetBlockRequest) (*GetBlockReply, error) {
	req.RequestType = "getBlock"
	return &req.res, w.processJSONRequest("GET", req, &req.res)
}

func (w *wallet) GetAccountsWithRewardRecipient(req *GetAccountsWithRewardRecipientRequest) (
	*GetAccountsWithRewardRecipientReply, error) {
	req.RequestType = "getAccountsWithRewardRecipient"
	return &req.res, w.processJSONRequest("POST", &req, &req.res)
}

func (w *wallet) SendMoney(req *SendMoneyRequest) (*SendMoneyReply, error) {
	req.RequestType = "sendMoney"
	return &req.res, w.processJSONRequest("POST", req, &req.res)
}

func (w *wallet) BroadcastTransaction(req *BroadcastTransactionRequest) (*BroadcastTransactionReply, error) {
	req.RequestType = "broadcastTransaction"
	return &req.res, w.processJSONRequest("POST", req, &req.res)
}

func (w *wallet) SendMoneyMulti(req *SendMoneyMultiRequest) (*SendMoneyMultiReply, error) {
	req.RequestType = "sendMoneyMulti"
	return &req.res, w.processJSONRequest("POST", req, &req.res)
}

func (w *wallet) GetAccountTransactions(req *GetAccountTransactionsRequest) (
	*GetAccountTransactionsReply, error) {
	req.RequestType = "getAccountTransactions"
	return &req.res, w.processJSONRequest("POST", req, &req.res)
}

func (w *wallet) GetAccount(req *GetAccountRequest) (*GetAccountReply, error) {
	req.RequestType = "getAccount"
	return &req.res, w.processJSONRequest("GET", req, &req.res)
}

func (w *wallet) GetTransaction(req *GetTransactionRequest) (*GetTransactionReply, error) {
	req.RequestType = "getTransaction"
	return &req.res, w.processJSONRequest("GET", req, &req.res)
}
