package zerodev

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	EntryPointVersion07 = "0.7"
	entrypointAbi07     = `[{"inputs": [{ "name": "sender", "type": "address" }, { "name": "key", "type": "uint192" }], "name": "getNonce", "outputs": [{ "name": "nonce", "type": "uint256" }], "stateMutability": "view", "type": "function"}]`
	entryPointAddress07 = "0x0000000071727De22E5E9d8BAf0edAc6f37da032"
)

const (
	keySeparatorStart = ">"
	keySeparatorEnd   = "<"
)

type Entrypoint interface {
	GetAddress() common.Address
	GetNonce(account common.Address) (*big.Int, error)
	GetUserOperationHash(op *UserOperation) (*common.Hash, error)
	PackUserOperation(op *UserOperation) ([]byte, error)
}

type EntrypointClient07 struct {
	Client  *rpc.Client
	Address common.Address
	Abi     *abi.ABI
	ChainID *big.Int
}

// NewEntrypoint07 creates a new EntrypointClient07 instance.
func NewEntrypoint07(rpcClient *rpc.Client, chainID *big.Int) (*EntrypointClient07, error) {
	parsedAbi, err := abi.JSON(strings.NewReader(entrypointAbi07))
	if err != nil {
		return nil, err
	}

	return &EntrypointClient07{
		Client:  rpcClient,
		Address: common.HexToAddress(entryPointAddress07),
		Abi:     &parsedAbi,
		ChainID: chainID,
	}, nil
}

func (e *EntrypointClient07) GetAddress() common.Address {
	return e.Address
}

// GetNonce retrieves the nonce of a specific account.
func (e *EntrypointClient07) GetNonce(account common.Address) (*big.Int, error) {
	key := computeKey(account)
	callData, err := e.Abi.Pack("getNonce", account, key)
	if err != nil {
		return nil, err
	}

	msg := struct {
		To   common.Address `json:"to"`
		Data hexutil.Bytes  `json:"data"`
	}{
		To:   e.Address,
		Data: callData,
	}

	var hex hexutil.Bytes
	if err := e.Client.CallContext(context.Background(), &hex, "eth_call", msg); err != nil {
		return nil, err
	}

	decoded, err := hexutil.Decode(hex.String())
	if err != nil {
		return nil, err
	}
	return big.NewInt(0).SetBytes(decoded), nil
}

// GetUserOperationHash calculates the hash of a UserOperation.
func (e *EntrypointClient07) GetUserOperationHash(op *UserOperation) (*common.Hash, error) {
	packedOp, err := e.PackUserOperation(op)
	if err != nil {
		return nil, err
	}

	args := abi.Arguments{
		{Type: bytes32},
		{Type: address},
		{Type: uint256},
	}

	packed, _ := args.Pack(
		crypto.Keccak256Hash(packedOp),
		e.Address,
		e.ChainID,
	)
	hash := crypto.Keccak256Hash(packed)
	return &hash, nil
}

// PackUserOperation creates a packed representation of a UserOperation compliant with Entrypoint 0.7
func (*EntrypointClient07) PackUserOperation(op *UserOperation) ([]byte, error) {
	args := abi.Arguments{
		{Name: "sender", Type: address},
		{Name: "nonce", Type: uint256},
		{Name: "hashInitCode", Type: bytes32},
		{Name: "hashCallData", Type: bytes32},
		{Name: "accountGasLimits", Type: bytes32},
		{Name: "preVerificationGas", Type: uint256},
		{Name: "gasFees", Type: bytes32},
		{Name: "hashPaymasterAndData", Type: bytes32},
	}

	hashedInitCode := crypto.Keccak256Hash(common.FromHex("0x"))
	hashedCallData := crypto.Keccak256Hash(op.CallData)

	accountGasLimits := createPackedBuffer(
		op.VerificationGasLimit.Bytes(),
		op.CallGasLimit.Bytes(),
	)

	gasFees := createPackedBuffer(
		op.MaxPriorityFeePerGas.Bytes(),
		op.MaxFeePerGas.Bytes(),
	)

	paymasterAndData := createPaymasterDataBuffer(
		op.Paymaster,
		op.PaymasterVerificationGasLimit.Bytes(),
		op.PaymasterPostOpGasLimit.Bytes(),
		op.PaymasterData,
	)

	hashedPaymasterAndData := crypto.Keccak256Hash(paymasterAndData.Bytes())

	packed, err := args.Pack(
		op.Sender,
		op.Nonce,
		hashedInitCode,
		hashedCallData,
		toArray32(accountGasLimits),
		op.PreVerificationGas,
		toArray32(gasFees),
		hashedPaymasterAndData,
	)
	if err != nil {
		return nil, err
	}
	return packed, nil
}

func (e *EntrypointClient07) Close() {
	e.Client.Close()
}

// computeKey generates a key for an account using separators.
func computeKey(account common.Address) *big.Int {
	partialHex := account.Hex()[5:10]
	return new(big.Int).SetBytes([]byte(keySeparatorStart + partialHex + keySeparatorEnd))
}

// createPackedBuffer combines two byte slices into a single buffer with padding.
func createPackedBuffer(first, second []byte) bytes.Buffer {
	var buffer bytes.Buffer
	buffer.Write(common.LeftPadBytes(first, 16))
	buffer.Write(common.LeftPadBytes(second, 16))
	return buffer
}

// createPaymasterDataBuffer builds the byte buffer for Paymaster and related data.
func createPaymasterDataBuffer(paymaster, verificationGas, postOpGas []byte, paymasterData []byte) bytes.Buffer {
	var buffer bytes.Buffer
	buffer.Write(paymaster)
	buffer.Write(common.LeftPadBytes(verificationGas, 16))
	buffer.Write(common.LeftPadBytes(postOpGas, 16))
	buffer.Write(paymasterData)
	return buffer
}

// toArray32 converts a buffer into a fixed 32-byte array.
func toArray32(buffer bytes.Buffer) [32]byte {
	var array [32]byte
	copy(array[:], buffer.Bytes())
	return array
}
