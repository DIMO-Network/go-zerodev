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
	var m = UserOperationHex{}
	m.Sender = op.Sender.String()
	m.Nonce = hexutil.EncodeBig(op.Nonce)
	m.CallData = hexutil.Encode(op.CallData)
	m.MaxFeePerGas = hexutil.EncodeBig(op.MaxFeePerGas)
	m.MaxPriorityFeePerGas = hexutil.EncodeBig(op.MaxPriorityFeePerGas)

	if op.CallGasLimit != nil {
		m.CallGasLimit = hexutil.EncodeBig(op.CallGasLimit)
	}

	if op.VerificationGasLimit != nil {
		m.VerificationGasLimit = hexutil.EncodeBig(op.VerificationGasLimit)
	}

	if op.PreVerificationGas != nil {
		m.PreVerificationGas = hexutil.EncodeBig(op.PreVerificationGas)
	}

	if len(op.Paymaster) > 0 {
		m.Paymaster = hexutil.Encode(op.Paymaster)
	}

	if len(op.PaymasterData) > 0 {
		m.PaymasterData = hexutil.Encode(op.PaymasterData)
	}

	if op.PaymasterPostOpGasLimit != nil {
		m.PaymasterPostOpGasLimit = hexutil.EncodeBig(op.PaymasterPostOpGasLimit)
	}

	if op.PaymasterVerificationGasLimit != nil {
		m.PaymasterVerificationGasLimit = hexutil.EncodeBig(op.PaymasterVerificationGasLimit)
	}

	if len(op.Signature) > 0 {
		m.Signature = hexutil.Encode(op.Signature)
	}

	return json.Marshal(&m)
}
