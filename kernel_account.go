package zerodev

import (
	"bytes"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const kernelAccountExecuteABI = `[{
        "type": "function",
        "name": "execute",
        "inputs": [
            { "name": "execMode", "type": "bytes32", "internalType": "ExecMode" },
            { "name": "executionCallData", "type": "bytes", "internalType": "bytes" }
        ],
        "outputs": [],
        "stateMutability": "payable"
    }]`

func EncodeExecuteCall(msg *ethereum.CallMsg) (*[]byte, error) {
	// based on https://github.com/zerodevapp/sdk/blob/main/packages/core/accounts/kernel/utils/ep0_7/encodeExecuteCall.ts#L24

	parsedABI, err := abi.JSON(strings.NewReader(kernelAccountExecuteABI))
	if err != nil {
		return nil, err
	}

	data := bytes.Buffer{}
	data.Write(msg.To.Bytes())
	data.Write(common.RightPadBytes(msg.Value.Bytes(), 32))
	data.Write(msg.Data)

	execMode := bytes.Buffer{}
	execMode.Write([]byte{0x00}) // call type
	execMode.Write([]byte{0x00}) // exec type
	execMode.Write([]byte{0x00000000})
	execMode.Write([]byte{0x00000000})
	execMode.Write(common.LeftPadBytes([]byte{0x00000000}, 32))

	var execModeArray [32]byte
	copy(execModeArray[:], execMode.Bytes())

	callData, err := parsedABI.Pack("execute", execModeArray, data.Bytes())
	if err != nil {
		return nil, err
	}

	return &callData, nil
}
