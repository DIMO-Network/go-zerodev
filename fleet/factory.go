package fleet

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Minimal ABIs — just what we need to encode the factory call data and the
// kernel initialize call data. Keeping these inline (vs importing a giant
// generated bindings package) makes it obvious that the bytes we produce
// are exactly what the SDK produces.

const kernelInitializeABI = `[{
    "type":"function","name":"initialize","stateMutability":"nonpayable",
    "inputs":[
        {"name":"_rootValidator","type":"bytes21"},
        {"name":"hook","type":"address"},
        {"name":"validatorData","type":"bytes"},
        {"name":"hookData","type":"bytes"},
        {"name":"initConfig","type":"bytes[]"}
    ],"outputs":[]
}]`

const metaFactoryDeployABI = `[{
    "type":"function","name":"deployWithFactory","stateMutability":"payable",
    "inputs":[
        {"name":"factory","type":"address"},
        {"name":"createData","type":"bytes"},
        {"name":"salt","type":"bytes32"}
    ],"outputs":[{"name":"","type":"address"}]
}]`

// FactoryData builds the bytes you put in UserOperation.FactoryData (with
// UserOperation.Factory = MetaFactoryAddress) for the first UserOp against
// a not-yet-deployed kernel.
//
// userSignerAddress is the EOA that owns the kernel's root validator (i.e.
// the address Turnkey signs with on the mobile-app side). index is the
// kernel salt — pass 0 unless the user has multiple kernels under one
// signer.
//
// The salt passed to deployWithFactory is the index as a 32-byte big-endian
// value; the actual CREATE2 salt the factory uses is hashed from this plus
// the init data, but that's the factory's internal concern.
func FactoryData(userSignerAddress common.Address, index *big.Int) ([]byte, error) {
	initData, err := encodeKernelInitialize(userSignerAddress)
	if err != nil {
		return nil, err
	}

	parsed, err := abi.JSON(strings.NewReader(metaFactoryDeployABI))
	if err != nil {
		return nil, err
	}

	var saltBytes [32]byte
	if index != nil {
		index.FillBytes(saltBytes[:])
	}

	return parsed.Pack("deployWithFactory", KernelFactoryAddress, initData, saltBytes)
}

// encodeKernelInitialize encodes kernel.initialize(rootValidator, hook,
// validatorData, hookData, initConfig) for our specific setup: ECDSA root
// validator, no hook, user address as validator data, no init config.
func encodeKernelInitialize(userSignerAddress common.Address) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(kernelInitializeABI))
	if err != nil {
		return nil, err
	}

	var rootVId [21]byte
	rootVId[0] = ValidatorTypeSecondary // root validator's type prefix in storage
	copy(rootVId[1:], EcdsaValidatorAddress.Bytes())

	return parsed.Pack(
		"initialize",
		rootVId,
		common.Address{},               // hook = zero
		userSignerAddress.Bytes(),      // validatorData = user EOA address
		[]byte{},                       // hookData = empty
		[][]byte{},                     // initConfig = empty (no extra init calls)
	)
}
