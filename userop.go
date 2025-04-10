package zerodev

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var (
	address, _ = abi.NewType("address", "", nil)
	uint256, _ = abi.NewType("uint256", "", nil)
	bytes32, _ = abi.NewType("bytes32", "", nil)
)

type UserOperation struct {
	Sender                        *common.Address `json:"sender"`
	Nonce                         *big.Int        `json:"nonce"`
	CallData                      []byte          `json:"callData"`
	CallGasLimit                  *big.Int        `json:"callGasLimit,omitempty"`
	VerificationGasLimit          *big.Int        `json:"verificationGasLimit,omitempty"`
	PreVerificationGas            *big.Int        `json:"preVerificationGas,omitempty"`
	MaxFeePerGas                  *big.Int        `json:"maxFeePerGas"`
	MaxPriorityFeePerGas          *big.Int        `json:"maxPriorityFeePerGas"`
	Paymaster                     []byte          `json:"paymaster,omitempty"`
	PaymasterData                 []byte          `json:"paymasterData,omitempty"`
	PaymasterVerificationGasLimit *big.Int        `json:"paymasterVerificationGasLimit,omitempty"`
	PaymasterPostOpGasLimit       *big.Int        `json:"paymasterPostOpGasLimit,omitempty"`
	Signature                     []byte          `json:"signature,omitempty"`
}

type UserOperationHex struct {
	Sender                        string `json:"sender"`
	Nonce                         string `json:"nonce"`
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

func encodeBigInt(value *big.Int) string {
	if value != nil {
		return hexutil.EncodeBig(value)
	}
	return ""
}

func encodeBytes(value []byte) string {
	if len(value) > 0 {
		return hexutil.Encode(value)
	}
	return ""
}
