package zerodev

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

var (
	address, _ = abi.NewType("address", "", nil)
	uint256, _ = abi.NewType("uint256", "", nil)
	bytes32, _ = abi.NewType("bytes32", "", nil)
)

type UserOperation struct {
	Sender                        common.Address `json:"sender"`
	Nonce                         *big.Int       `json:"nonce"`
	// Factory + FactoryData are set on the first UserOp against an
	// undeployed account so the EntryPoint deploys it before validation.
	// Factory must be the meta-factory address (20 bytes); FactoryData is
	// the call data passed to that factory. Leave both nil when the
	// account is already deployed.
	Factory                       []byte         `json:"factory,omitempty"`
	FactoryData                   []byte         `json:"factoryData,omitempty"`
	CallData                      []byte         `json:"callData"`
	CallGasLimit                  *big.Int       `json:"callGasLimit,omitempty"`
	VerificationGasLimit          *big.Int       `json:"verificationGasLimit,omitempty"`
	PreVerificationGas            *big.Int       `json:"preVerificationGas,omitempty"`
	MaxFeePerGas                  *big.Int       `json:"maxFeePerGas"`
	MaxPriorityFeePerGas          *big.Int       `json:"maxPriorityFeePerGas"`
	Paymaster                     []byte         `json:"paymaster,omitempty"`
	PaymasterData                 []byte         `json:"paymasterData,omitempty"`
	PaymasterVerificationGasLimit *big.Int       `json:"paymasterVerificationGasLimit,omitempty"`
	PaymasterPostOpGasLimit       *big.Int       `json:"paymasterPostOpGasLimit,omitempty"`
	Signature                     []byte         `json:"signature,omitempty"`
}

type UserOperationHex struct {
	Sender                        string `json:"sender"`
	Nonce                         string `json:"nonce"`
	Factory                       string `json:"factory,omitempty"`
	FactoryData                   string `json:"factoryData,omitempty"`
	CallData                      string `json:"callData"`
	CallGasLimit                  string `json:"callGasLimit,omitempty"`
	VerificationGasLimit          string `json:"verificationGasLimit,omitempty"`
	PreVerificationGas            string `json:"preVerificationGas,omitempty"`
	MaxFeePerGas                  string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas          string `json:"maxPriorityFeePerGas"`
	Paymaster                     string `json:"paymaster,omitempty"`
	PaymasterData                 string `json:"paymasterData,omitempty"`
	PaymasterVerificationGasLimit string `json:"paymasterVerificationGasLimit,omitempty"`
	PaymasterPostOpGasLimit       string `json:"paymasterPostOpGasLimit,omitempty"`
	Signature                     string `json:"signature,omitempty"`
}

func (op *UserOperation) MarshalJSON() ([]byte, error) {
	hexOp := UserOperationHex{
		Sender:                        op.Sender.String(),
		Nonce:                         encodeBigInt(op.Nonce),
		Factory:                       encodeBytes(op.Factory),
		FactoryData:                   encodeBytes(op.FactoryData),
		CallData:                      encodeBytes(op.CallData),
		MaxFeePerGas:                  encodeBigInt(op.MaxFeePerGas),
		MaxPriorityFeePerGas:          encodeBigInt(op.MaxPriorityFeePerGas),
		CallGasLimit:                  encodeBigInt(op.CallGasLimit),
		VerificationGasLimit:          encodeBigInt(op.VerificationGasLimit),
		PreVerificationGas:            encodeBigInt(op.PreVerificationGas),
		Paymaster:                     encodeBytes(op.Paymaster),
		PaymasterData:                 encodeBytes(op.PaymasterData),
		Signature:                     encodeBytes(op.Signature),
		PaymasterPostOpGasLimit:       encodeBigInt(op.PaymasterPostOpGasLimit),
		PaymasterVerificationGasLimit: encodeBigInt(op.PaymasterVerificationGasLimit),
	}
	return json.Marshal(&hexOp)
}

func (op *UserOperation) UnmarshalJSON(b []byte) error {
	var hexOp UserOperationHex
	err := json.Unmarshal(b, &hexOp)
	if err != nil {
		return err
	}

	op.Sender = common.HexToAddress(hexOp.Sender)

	op.Nonce, err = decodeBigInt(hexOp.Nonce)
	if err != nil {
		return err
	}

	op.Factory, err = decodeBytes(hexOp.Factory)
	if err != nil {
		return err
	}

	op.FactoryData, err = decodeBytes(hexOp.FactoryData)
	if err != nil {
		return err
	}

	op.CallData, err = decodeBytes(hexOp.CallData)
	if err != nil {
		return err
	}

	op.MaxFeePerGas, err = decodeBigInt(hexOp.MaxFeePerGas)
	if err != nil {
		return err
	}

	op.MaxPriorityFeePerGas, err = decodeBigInt(hexOp.MaxPriorityFeePerGas)
	if err != nil {
		return err
	}

	op.CallGasLimit, err = decodeBigInt(hexOp.CallGasLimit)
	if err != nil {
		return err
	}

	op.VerificationGasLimit, err = decodeBigInt(hexOp.VerificationGasLimit)
	if err != nil {
		return err
	}

	op.PreVerificationGas, err = decodeBigInt(hexOp.PreVerificationGas)
	if err != nil {
		return err
	}

	op.Paymaster, err = decodeBytes(hexOp.Paymaster)
	if err != nil {
		return err
	}

	op.PaymasterData, err = decodeBytes(hexOp.PaymasterData)
	if err != nil {
		return err
	}

	op.Signature, err = decodeBytes(hexOp.Signature)
	if err != nil {
		return err
	}

	op.PaymasterPostOpGasLimit, err = decodeBigInt(hexOp.PaymasterPostOpGasLimit)
	if err != nil {
		return err
	}

	op.PaymasterVerificationGasLimit, err = decodeBigInt(hexOp.PaymasterVerificationGasLimit)
	if err != nil {
		return err
	}

	return nil
}

func encodeBigInt(value *big.Int) string {
	if value != nil {
		return hexutil.EncodeBig(value)
	}
	return ""
}

func decodeBigInt(value string) (*big.Int, error) {
	if value != "" {
		return hexutil.DecodeBig(value)
	}
	return nil, nil
}

func encodeBytes(value []byte) string {
	if len(value) > 0 {
		return hexutil.Encode(value)
	}
	return ""
}

func decodeBytes(value string) ([]byte, error) {
	if value != "" {
		return hexutil.Decode(value)
	}
	return nil, nil
}
