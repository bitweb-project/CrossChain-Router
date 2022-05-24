package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/common"
	"github.com/anyswap/CrossChain-Router/v3/log"
	"github.com/anyswap/CrossChain-Router/v3/mpc"
	"github.com/anyswap/CrossChain-Router/v3/params"
	"github.com/anyswap/CrossChain-Router/v3/tokens"
	"github.com/anyswap/CrossChain-Router/v3/tokens/near"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"
)

var (
	bridge = near.NewCrossChainBridge()

	paramConfigFile     string
	paramChainID        string
	paramFunctionName   string
	paramPublicKey      string
	paramRouterContract string
	paramNewMpcId       string
	paramNewGas         string
	paramToken          string
	paramTxHash         string
	paramTo             string
	paramAmount         string
	paramFromChainId    string
	paramGas            uint64 = 300_000_000_000_000
	chainID                    = big.NewInt(0)
	mpcConfig           *mpc.Config
	supportFuncionList  = make(map[string]bool)
	changeMPC           = "change_mpc_id"
	setBaseGas          = "set_base_gas"
	setGas              = "set_gas"
	anySwapInAll        = "any_swap_in_all"
)

func main() {
	log.SetLogger(6, false, true)

	initAll()

	if !supportFuncionList[paramFunctionName] {
		log.Fatal("call function name not support")
		return
	}
	var err error
	var nearPubKey *near.PublicKey
	if len(paramPublicKey) == 64 {
		nearPubKey, err = near.PublicKeyFromHexString(paramPublicKey)
		if err != nil {
			log.Fatal("convert public key to address failed")
		}
	} else {
		log.Fatal("len of public key not match")
	}

	nonce, err := bridge.GetAccountNonce(nearPubKey.Address(), nearPubKey.String())
	if err != nil {
		log.Fatal("get account nonce failed", "err", err)
	}

	blockHash, err := bridge.GetLatestBlockHash()
	if err != nil {
		log.Fatal("get last block hash failed", "err", err)
	}
	blockHashBytes, err := base58.Decode(blockHash)
	if err != nil {
		log.Fatal("get last block hash failed", "err", err)
	}

	log.Info("get account nonce success", "nonce", nonce)
	actions, err := createFunctionCall()
	if err != nil {
		log.Fatal("createFunctionCall failed", "err", err)
	}
	rawTx := near.CreateTransaction(nearPubKey.Address(), nearPubKey, paramRouterContract, nonce, blockHashBytes, actions)
	signedTx, txHash, err := MPCSignTransaction(rawTx, paramPublicKey)
	if err != nil {
		log.Fatal("sign tx failed", "err", err)
	}
	log.Info("sign tx success", "txHash", txHash)

	txHash, err = bridge.SendTransaction(signedTx)
	if err != nil {
		log.Fatal("send tx failed", "err", err)
	}
	log.Info("send tx success", "txHash", txHash)
}

//nolint:gocyclo // allow long method
func createFunctionCall() ([]near.Action, error) {
	log.Info("createFunctionCall", "methodName", paramFunctionName)
	var argsBytes []byte
	switch paramFunctionName {
	case changeMPC:
		if paramNewMpcId == "" {
			return nil, errors.New("paramNewMpcId must input")
		}
		argsBytes = changeMpcArgs(paramNewMpcId)
	case setBaseGas:
		if paramNewGas == "" {
			return nil, errors.New("paramNewGas must input")
		}
		newGas, err := common.GetUint64FromStr(paramNewGas)
		if err != nil {
			return nil, err
		}
		argsBytes = setBaseGasArgs(newGas)
	case setGas:
		if paramNewGas == "" || paramToken == "" {
			return nil, errors.New("paramToken and paramNewGas must input")
		}
		newGas, err := common.GetUint64FromStr(paramNewGas)
		if err != nil {
			return nil, err
		}
		argsBytes = setGasArgs(paramToken, newGas)
	case anySwapInAll:
		if paramTxHash == "" || paramToken == "" || paramTo == "" || paramAmount == "" || paramFromChainId == "" {
			return nil, errors.New("paramTxHash,paramToken,paramTo,paramAmount and paramFromChainId must input")
		}
		argsBytes = anySwapInAllArgs(paramTxHash, paramToken, paramTo, paramAmount, paramFromChainId)
	default:
		log.Fatalf("unknown method name: '%v'", paramFunctionName)
	}
	return []near.Action{{
		Enum: 2,
		FunctionCall: near.FunctionCall{
			MethodName: paramFunctionName,
			Args:       argsBytes,
			Gas:        paramGas,
			Deposit:    *big.NewInt(0),
		},
	}}, nil
}

func changeMpcArgs(newMpcId string) []byte {
	callArgs := &near.ChangeMpcId{
		NewMpcId: newMpcId,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}

func setBaseGasArgs(gas uint64) []byte {
	callArgs := &near.SetBaseGas{
		Gas: gas,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}

func setGasArgs(token string, gas uint64) []byte {
	callArgs := &near.SetGas{
		Token: token,
		Gas:   gas,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}

func anySwapInAllArgs(txHash, token, to, amount, fromChainId string) []byte {
	callArgs := &near.AnySwapInAll{
		TxHash:      txHash,
		Token:       token,
		To:          to,
		Amount:      amount,
		FromChainId: fromChainId,
	}
	argsBytes, _ := json.Marshal(callArgs)
	return argsBytes
}

func MPCSignTransaction(rawTx interface{}, paramPublicKey string) (signedTx interface{}, txHash string, err error) {
	tx, ok := rawTx.(*near.RawTransaction)
	if !ok {
		return nil, "", tokens.ErrWrongRawTx
	}

	buf, err := borsh.Serialize(*tx)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256(buf)

	keyID, rsvs, err := mpcConfig.DoSignOneED(paramPublicKey, common.ToHex(hash[:]), "")
	if err != nil {
		return nil, "", err
	}

	if len(rsvs) != 1 {
		log.Warn("get sign status require one rsv but return many",
			"rsvs", len(rsvs), "keyID", keyID)
		return nil, "", errors.New("get sign status require one rsv but return many")
	}

	rsv := rsvs[0]
	log.Trace("get rsv signature success", "keyID", keyID, "rsv", rsv)
	sig := common.FromHex(rsv)
	if len(sig) != ed25519.SignatureSize {
		log.Error("wrong signature length", "keyID", keyID, "have", len(sig), "want", ed25519.SignatureSize)
		return nil, "", errors.New("wrong signature length")
	}

	var signature near.Signature
	signature.KeyType = 0
	copy(signature.Data[:], sig)

	var stx near.SignedTransaction
	stx.Transaction = *tx
	stx.Signature = signature

	txHash = base58.Encode(hash[:])

	log.Info("success", "keyID", keyID, "txhash", txHash)
	return &stx, txHash, nil
}

func initAll() {
	initFlags()
	initConfig()
	initBridge()
	initSupportList()
}

func initFlags() {
	flag.StringVar(&paramConfigFile, "config", "", "config file to init mpc and gateway")
	flag.StringVar(&paramChainID, "chainID", "", "chain id")
	flag.StringVar(&paramRouterContract, "routerContract", "", "router contract address")
	flag.StringVar(&paramFunctionName, "functionName", "", "function name")
	flag.StringVar(&paramPublicKey, "pubKey", "", "signer public key")
	flag.StringVar(&paramNewMpcId, "newMpcId", "", "(optional) new mpc id")
	flag.StringVar(&paramNewGas, "newGas", "", "(optional) new gas")
	flag.StringVar(&paramToken, "token", "", "(optional) token contract address")
	flag.StringVar(&paramTxHash, "txHash", "", "(optional) tx hash")
	flag.StringVar(&paramTo, "to", "", "(optional) receive address")
	flag.StringVar(&paramAmount, "amount", "", "(optional) receive amount")
	flag.StringVar(&paramFromChainId, "fromChainId", "", "(optional) from chain id")

	flag.Parse()

	if paramChainID != "" {
		cid, err := common.GetBigIntFromStr(paramChainID)
		if err != nil {
			log.Fatal("wrong param chainID", "err", err)
		}
		chainID = cid
	}

	log.Info("init flags finished", "functionName", paramFunctionName)
}

func initConfig() {
	config := params.LoadRouterConfig(paramConfigFile, true, false)
	mpcConfig = mpc.InitConfig(config.MPC, true)
	log.Info("init config finished")
}

func initBridge() {
	cfg := params.GetRouterConfig()
	apiAddrs := cfg.Gateways[chainID.String()]
	if len(apiAddrs) == 0 {
		log.Fatal("gateway not found for chain ID", "chainID", chainID)
	}
	apiAddrsExt := cfg.GatewaysExt[chainID.String()]
	bridge.SetGatewayConfig(&tokens.GatewayConfig{
		APIAddress:    apiAddrs,
		APIAddressExt: apiAddrsExt,
	})
	log.Info("init bridge finished")
}

func initSupportList() {
	supportFuncionList[changeMPC] = true
	supportFuncionList[setBaseGas] = true
	supportFuncionList[setGas] = true
	supportFuncionList[anySwapInAll] = true
}
