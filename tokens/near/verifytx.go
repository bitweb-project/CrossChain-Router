package near

import (
	"crypto/sha256"
	"errors"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/router"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"
)

var (
	errTxResultType = errors.New("tx type is not TransactionResult")
	errTxLogParse   = errors.New("tx logs is not LogSwapOut")
	logFile         = "LogSwapOut"
)

// VerifyMsgHash verify msg hash
func (b *Bridge) VerifyMsgHash(rawTx interface{}, msgHashes []string) (err error) {
	txb, ok := rawTx.(*RawTransaction)
	if !ok {
		return tokens.ErrWrongRawTx
	}
	buf, errb := borsh.Serialize(*txb)
	if errb != nil {
		return errb
	}

	hash := sha256.Sum256(buf)

	if len(msgHashes) < 1 {
		return tokens.ErrWrongCountOfMsgHashes
	}
	msgHash := msgHashes[0]
	sigHash := base58.Encode(hash[:])

	if !strings.EqualFold(sigHash, msgHash) {
		logFunc := log.GetPrintFuncOr(params.IsDebugMode, log.Info, log.Trace)
		logFunc("message hash mismatch", "want", msgHash, "have", sigHash)
		return tokens.ErrMsgHashMismatch
	}
	return nil
}

// VerifyTransaction impl
func (b *Bridge) VerifyTransaction(txHash string, args *tokens.VerifyArgs) (*tokens.SwapTxInfo, error) {
	swapType := args.SwapType
	logIndex := args.LogIndex
	allowUnstable := args.AllowUnstable

	switch swapType {
	case tokens.ERC20SwapType:
		return b.verifySwapoutTx(txHash, logIndex, allowUnstable)
	default:
		return nil, tokens.ErrSwapTypeNotSupported
	}
}

func (b *Bridge) verifySwapoutTx(txHash string, logIndex int, allowUnstable bool) (*tokens.SwapTxInfo, error) {
	swapInfo := &tokens.SwapTxInfo{SwapInfo: tokens.SwapInfo{ERC20SwapInfo: &tokens.ERC20SwapInfo{}}}
	swapInfo.SwapType = tokens.ERC20SwapType          // SwapType
	swapInfo.Hash = strings.ToLower(txHash)           // Hash
	swapInfo.LogIndex = logIndex                      // LogIndex
	swapInfo.FromChainID = b.ChainConfig.GetChainID() // FromChainID

	tx, txErr := b.GetTransaction(txHash)
	if txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", txHash, "err", txErr)
		return swapInfo, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*TransactionResult)
	if !ok {
		return swapInfo, errTxResultType
	}

	statusErr := b.checkTxStatus(txres, allowUnstable)
	if statusErr != nil {
		return swapInfo, statusErr
	}

	events := fliterReceipts(txres.ReceiptsOutcome, b.ChainConfig.RouterContract)
	event, fliterErr := fliterEvent(events)
	if fliterErr != nil {
		return swapInfo, errTxLogParse
	}

	parseErr := b.parseNep141SwapoutTxEvent(swapInfo, event)
	if parseErr != nil {
		return swapInfo, parseErr
	}

	checkErr := b.checkSwapoutInfo(swapInfo)
	if checkErr != nil {
		return swapInfo, checkErr
	}

	if !allowUnstable {
		log.Info("verify swapout pass",
			"token", swapInfo.ERC20SwapInfo.Token, "from", swapInfo.From, "to", swapInfo.To,
			"bind", swapInfo.Bind, "value", swapInfo.Value, "txid", swapInfo.Hash,
			"height", swapInfo.Height, "timestamp", swapInfo.Timestamp, "logIndex", swapInfo.LogIndex)
	}

	return swapInfo, nil
}

func fliterReceipts(receipts []ReceiptsOutcome, routerAddr string) (logs []string) {
	for i := 0; i < len(receipts); i++ {
		receipt := &receipts[i]
		if receipt.Outcome.ExecutorID == routerAddr {
			logs = append(logs, receipt.Outcome.Logs...)
		}
	}
	return
}

func fliterEvent(logs []string) ([]string, error) {
	for _, log := range logs {
		words := strings.Fields(log)
		if len(words) == 13 && words[0] == logFile {
			return words, nil
		}
	}
	return nil, errTxLogParse
}

func (b *Bridge) checkTxStatus(txres *TransactionResult, allowUnstable bool) error {
	if txres.Status.Failure != nil {
		return tokens.ErrTxIsNotValidated
	}

	if !allowUnstable {
		lastHeight, errh1 := b.GetLatestBlockNumber()
		if errh1 != nil {
			return errh1
		}

		txHeight, errh2 := b.GetBlockNumberByHash(txres.TransactionOutcome.BlockHash)
		if errh2 != nil {
			return errh2
		}

		if lastHeight < txHeight+b.GetChainConfig().Confirmations {
			return tokens.ErrTxNotStable
		}

		if lastHeight < b.ChainConfig.InitialHeight {
			return tokens.ErrTxBeforeInitialHeight
		}
	}
	return nil
}

func (b *Bridge) parseNep141SwapoutTxEvent(swapInfo *tokens.SwapTxInfo, event []string) error {
	swapInfo.ERC20SwapInfo.Token = event[2]
	swapInfo.From = event[4]
	swapInfo.Bind = event[6]

	amount, erra := common.GetBigIntFromStr(event[8])
	if erra != nil {
		return erra
	}
	swapInfo.Value = amount

	toChainID, errt := common.GetBigIntFromStr(event[12])
	if errt != nil {
		return errt
	}
	swapInfo.ToChainID = toChainID

	tokenCfg := b.GetTokenConfig(swapInfo.ERC20SwapInfo.Token)
	if tokenCfg == nil {
		return tokens.ErrMissTokenConfig
	}
	swapInfo.ERC20SwapInfo.TokenID = tokenCfg.TokenID

	depositAddress := b.GetRouterContract(swapInfo.ERC20SwapInfo.Token)
	swapInfo.To = depositAddress
	return nil
}

func (b *Bridge) checkSwapoutInfo(swapInfo *tokens.SwapTxInfo) error {
	if strings.EqualFold(swapInfo.From, swapInfo.To) {
		return tokens.ErrTxWithWrongSender
	}

	erc20SwapInfo := swapInfo.ERC20SwapInfo

	fromTokenCfg := b.GetTokenConfig(erc20SwapInfo.Token)
	if fromTokenCfg == nil || erc20SwapInfo.TokenID == "" {
		return tokens.ErrMissTokenConfig
	}

	multichainToken := router.GetCachedMultichainToken(erc20SwapInfo.TokenID, swapInfo.ToChainID.String())
	if multichainToken == "" {
		log.Warn("get multichain token failed", "tokenID", erc20SwapInfo.TokenID, "chainID", swapInfo.ToChainID, "txid", swapInfo.Hash)
		return tokens.ErrMissTokenConfig
	}

	toBridge := router.GetBridgeByChainID(swapInfo.ToChainID.String())
	if toBridge == nil {
		return tokens.ErrNoBridgeForChainID
	}

	toTokenCfg := toBridge.GetTokenConfig(multichainToken)
	if toTokenCfg == nil {
		log.Warn("get token config failed", "chainID", swapInfo.ToChainID, "token", multichainToken)
		return tokens.ErrMissTokenConfig
	}

	if !tokens.CheckTokenSwapValue(swapInfo, fromTokenCfg.Decimals, toTokenCfg.Decimals) {
		return tokens.ErrTxWithWrongValue
	}

	bindAddr := swapInfo.Bind
	if !toBridge.IsValidAddress(bindAddr) {
		log.Warn("wrong bind address in swapin", "bind", bindAddr)
		return tokens.ErrWrongBindAddress
	}
	return nil
}

func (b *Bridge) getSwapTxReceipt(swapInfo *tokens.SwapTxInfo, allowUnstable bool) ([]string, error) {
	tx, txErr := b.GetTransaction(swapInfo.Hash)
	if txErr != nil {
		log.Debug("[verifySwapout] "+b.ChainConfig.BlockChain+" Bridge::GetTransaction fail", "tx", swapInfo.Hash, "err", txErr)
		return nil, tokens.ErrTxNotFound
	}

	txres, ok := tx.(*TransactionResult)
	if !ok {
		return nil, errTxResultType
	}

	statusErr := b.checkTxStatus(txres, allowUnstable)
	if statusErr != nil {
		return nil, statusErr
	}

	events := fliterReceipts(txres.ReceiptsOutcome, b.ChainConfig.RouterContract)
	event, fliterErr := fliterEvent(events)
	if fliterErr != nil {
		return nil, errTxLogParse
	}
	return event, nil
}
