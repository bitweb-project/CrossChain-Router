package serialize

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/anyswap/CrossChain-Router/v3/tokens/near/utils"
	"github.com/shopspring/decimal"
)

/*
fork: github.com/near/near-api-js/src/transaction.ts  --> SCHEMA
fork: github.com/near/near-api-py/near_api/transactions.py -->tx_schema
*/
const (
	CreateAccountAction = iota
	DeployContractAction
	FunctionCallAction
	TransferAction
	StakeAction
	AddKey
	DeleteKey
	DeleteAccount
)

type ISerialize interface {
	Serialize() ([]byte, error)
}

type IAction interface {
	ISerialize
	GetActionIndex() uint8
}

type U8 struct {
	Value uint8
}

func (u *U8) Serialize() ([]byte, error) {
	return []byte{u.Value}, nil
}

type U32 struct {
	Value uint32
}

func (u *U32) Serialize() ([]byte, error) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, u.Value)
	return data, nil
}

type U64 struct {
	Value uint64
}

func (u *U64) Serialize() ([]byte, error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, u.Value)
	return data, nil
}

type U128 struct {
	Value *big.Int
}

func (u *U128) Serialize() ([]byte, error) {
	data, err := utils.BigIntToUintBytes(u.Value, 16)
	if err != nil {
		return nil, err
	}
	utils.Reverse(data)
	return data, nil
}

type String struct {
	Value string
}

func (s *String) Serialize() ([]byte, error) {
	if s.Value == "" {
		return nil, errors.New("string is null")
	}
	length := len(s.Value)
	uL := U32{
		Value: uint32(length),
	}

	data, err := uL.Serialize()
	if err != nil {
		return nil, err
	}
	data = append(data, []byte(s.Value)...)
	return data, nil
}

type PublicKey struct {
	KeyType uint8
	Value   []byte
}

func (s *PublicKey) Serialize() ([]byte, error) {
	data := []byte{s.KeyType}
	if len(s.Value) != 32 {
		return nil, fmt.Errorf("publickey length is not equal 32,length=%d", len(s.Value))
	}
	data = append(data, s.Value...)
	return data, nil
}

type Signature struct {
	KeyType uint8
	Value   []byte
}

func (s *Signature) Serialize() ([]byte, error) {
	data := []byte{s.KeyType}
	if len(s.Value) != 64 {
		return nil, fmt.Errorf("signature length is not equal 64,length=%d", len(s.Value))
	}
	data = append(data, s.Value...)
	return data, nil
}

type BlockHash struct {
	Value []byte
}

func (s *BlockHash) Serialize() ([]byte, error) {
	if len(s.Value) != 32 {
		return nil, fmt.Errorf("blockhash length is not equal 32,length=%d", len(s.Value))
	}
	return s.Value, nil
}

type FunctionCall struct {
	Action     uint8
	MethodName String
	Args       []byte
	Gas        U64
	Deposit    String
}

func CreateFuncCall(method string, args []byte, gas uint64, deposit string) (*FunctionCall, error) {
	return &FunctionCall{
		Action:     FunctionCallAction,
		MethodName: String{Value: method},
		Args:       args,
		Gas:        U64{Value: gas},
		Deposit:    String{Value: deposit},
	}, nil
}

func (s *FunctionCall) GetActionIndex() uint8 {
	return s.Action
}

func (s *FunctionCall) Serialize() ([]byte, error) {
	data := []byte{s.Action}

	v, err := s.MethodName.Serialize()
	if err != nil {
		return nil, err
	}
	data = append(data, v...)

	data = append(data, s.Args...)

	v1, err1 := s.Gas.Serialize()
	if err1 != nil {
		return nil, err1
	}
	data = append(data, v1...)

	v2, err2 := s.Deposit.Serialize()
	if err2 != nil {
		return nil, err2
	}
	data = append(data, v2...)

	return data, nil
}

//Action
type Transfer struct {
	Action uint8
	Value  U128
}

func CreateTransfer(amount string) (*Transfer, error) {
	dec, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	return &Transfer{
		Action: TransferAction,
		Value:  U128{Value: dec.BigInt()},
	}, nil
}
func (s *Transfer) GetActionIndex() uint8 {
	return s.Action
}
func (s *Transfer) Serialize() ([]byte, error) {
	data := []byte{s.Action}
	v, err := s.Value.Serialize()
	if err != nil {
		return nil, err
	}
	data = append(data, v...)
	return data, nil
}

type CreateAccount struct {
	Action uint8
}

func (s *CreateAccount) GetActionIndex() uint8 {
	return s.Action
}

func (s *CreateAccount) Serialize() ([]byte, error) {
	return []byte{s.Action}, nil
}
