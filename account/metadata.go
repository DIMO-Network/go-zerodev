package account

import (
	"context"
	"github.com/DIMO-Network/go-zerodev"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"strings"
)

type AccountMetadata struct {
	Fields            [1]byte        `json:"fields"`
	Name              string         `json:"name"`
	Version           string         `json:"version"`
	ChainId           *big.Int       `json:"chainId"`
	VerifyingContract common.Address `json:"verifyingContract"`
	Salt              [32]byte       `json:"salt"`
	Extensions        []*big.Int     `json:"extensions"`
}

const Eip1271Abi = `[
    {
        "type": "function",
        "name": "eip712Domain",
        "inputs": [],
        "outputs": [
            { "name": "fields", "type": "bytes1", "internalType": "bytes1" },
            { "name": "name", "type": "string", "internalType": "string" },
            { "name": "version", "type": "string", "internalType": "string" },
            { "name": "chainId", "type": "uint256", "internalType": "uint256" },
            {
                "name": "verifyingContract",
                "type": "address",
                "internalType": "address"
            },
            { "name": "salt", "type": "bytes32", "internalType": "bytes32" },
            { "name": "extensions", "type": "uint256[]", "internalType": "uint256[]" }
        ],
        "stateMutability": "view"
    },
    {
        "type": "function",
        "name": "isValidSignature",
        "inputs": [
            { "name": "data", "type": "bytes32", "internalType": "bytes32" },
            { "name": "signature", "type": "bytes", "internalType": "bytes" }
        ],
        "outputs": [
            { "name": "magicValue", "type": "bytes4", "internalType": "bytes4" }
        ],
        "stateMutability": "view"
    }
]`

func GetAccountMetadata(client zerodev.RPCClient, address common.Address) (*AccountMetadata, error) {
	parsedAbi, err := abi.JSON(strings.NewReader(Eip1271Abi))
	if err != nil {
		return nil, err
	}

	callData, err := parsedAbi.Pack("eip712Domain")
	if err != nil {
		return nil, err
	}

	msg := struct {
		To   common.Address `json:"to"`
		Data hexutil.Bytes  `json:"data"`
	}{
		To:   address,
		Data: callData,
	}

	var hex hexutil.Bytes
	if err := client.CallContext(context.Background(), &hex, "eth_call", msg, "latest"); err != nil {
		return nil, err
	}

	var accountMetadata AccountMetadata
	err = parsedAbi.UnpackIntoInterface(&accountMetadata, "eip712Domain", hex)
	if err != nil {
		return nil, err
	}

	return &accountMetadata, nil
}
