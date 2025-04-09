package zerodev

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	EntryPointVersion07 = "0.7"
)

type Entrypoint interface {
	GetAddress() *common.Address
	GetNonce(account *common.Address) (*big.Int, error)
	GetUserOperationHash(op *UserOperation) (*common.Hash, error)
	Close()
}

type EntrypointClient07 struct {
	Client  *rpc.Client
	Address *common.Address
	Abi     *abi.ABI
	ChainID *big.Int
}

const entrypointAbi07 = `[{"inputs": [{ "name": "sender", "type": "address" }, { "name": "key", "type": "uint192" }], "name": "getNonce", "outputs": [{ "name": "nonce", "type": "uint256" }], "stateMutability": "view", "type": "function"}]`
const entryPointAddress07 = "0x0000000071727De22E5E9d8BAf0edAc6f37da032"

func NewEntrypoint07(rpcUrl *url.URL, chainID *big.Int) (*EntrypointClient07, error) {
	rpcClient, err := rpc.Dial(rpcUrl.String())
	if err != nil {
		return nil, err
	}

	parsedABI, err := abi.JSON(strings.NewReader(entrypointAbi07))
	if err != nil {
		return nil, err
	}

	entrypointAddress := common.HexToAddress(entryPointAddress07)

	return &EntrypointClient07{
		Client:  rpcClient,
		Address: &entrypointAddress,
		Abi:     &parsedABI,
		ChainID: chainID,
	}, nil
}

func (e *EntrypointClient07) GetAddress() *common.Address {
	return e.Address
}

func (e *EntrypointClient07) GetNonce(account *common.Address) (*big.Int, error) {
	key := new(big.Int).SetBytes([]byte(">" + account.Hex()[5:10] + "<"))
	callData, err := e.Abi.Pack("getNonce", account, key)
	if err != nil {
		return nil, err
	}

	msg := struct {
		To   *common.Address `json:"to"`
		Data hexutil.Bytes   `json:"data"`
	}{
		To:   e.Address,
		Data: callData,
	}

	var hex hexutil.Bytes
	err = e.Client.CallContext(context.Background(), &hex, "eth_call", msg)
	if err != nil {
		return nil, err
	}

	decoded, err := hexutil.Decode(hex.String())
	if err != nil {
		return nil, err
	}

	return big.NewInt(0).SetBytes(decoded), nil
}

func (e *EntrypointClient07) GetUserOperationHash(op *UserOperation) (*common.Hash, error) {
	packedOp, err := e.PackUserOperation(op)
	args := abi.Arguments{
		{Type: bytes32},
		{Type: address},
		{Type: uint256},
	}

	if err != nil {
		return nil, err
	}

	packed, _ := args.Pack(
		crypto.Keccak256Hash(packedOp),
		e.Address,
		e.ChainID,
	)

	hash := crypto.Keccak256Hash(packed)

	return &hash, nil
}

func (*EntrypointClient07) PackUserOperation(op *UserOperation) ([]byte, error) {
	// Based on:
	// https://github.com/wevm/viem/blob/main/src/account-abstraction/utils/userOperation/getUserOperationHash.ts#L72
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

	var accountGasLimits bytes.Buffer
	accountGasLimits.Write(common.LeftPadBytes(op.VerificationGasLimit.Bytes(), 16))
	accountGasLimits.Write(common.LeftPadBytes(op.CallGasLimit.Bytes(), 16))

	var accountGasLimitsArray [32]byte
	copy(accountGasLimitsArray[:], accountGasLimits.Bytes())

	var gasFees bytes.Buffer
	gasFees.Write(common.LeftPadBytes(op.MaxPriorityFeePerGas.Bytes(), 16))
	gasFees.Write(common.LeftPadBytes(op.MaxFeePerGas.Bytes(), 16))

	var gasFeesArray [32]byte
	copy(gasFeesArray[:], gasFees.Bytes())

	var paymasterAndData bytes.Buffer
	paymasterAndData.Write(op.Paymaster)
	paymasterAndData.Write(common.LeftPadBytes(op.PaymasterVerificationGasLimit.Bytes(), 16))
	paymasterAndData.Write(common.LeftPadBytes(op.PaymasterPostOpGasLimit.Bytes(), 16))
	paymasterAndData.Write(op.PaymasterData)

	hashedPaymasterAndData := crypto.Keccak256Hash(paymasterAndData.Bytes())

	packed, err := args.Pack(
		op.Sender,
		op.Nonce,
		hashedInitCode,
		hashedCallData,
		accountGasLimitsArray,
		op.PreVerificationGas,
		gasFeesArray,
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
