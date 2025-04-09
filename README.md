# go-zerodev

Basic Go SDK for executing transactions with zerodev paymaster and bundler

## Limitations

- Only entrypoint 0.7 is supported
- AA wallet has to be already deployed, the SDK does not support walled deployment at this point
- Only single call is supported

## Usage

```go
package main

import (
	"fmt"
	"math/big"

	"github.com/DIMO-Network/go-zerodev"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// Create a signer for the AA wallet (only private key is supported right now)
	signer := zerodev.NewPrivateKeySigner(<YOUR_AA_WALLET_ECDSA_PK>)
	walletAddress := common.HexToAddress("YOUR_AA_WALLET_ADDRESS")

	// Create config for zerodev client
	clientConfig := zerodev.ClientConfig{
		Signer:             signer,
		EntryPointVersion:  zerodev.EntryPointVersion07,
		RpcURL:             <RPC_URL>,
		PaymasterURL:       <PAYMASTER_URL>,
		BundlerURL:         <BUNDLER_URL>,
		ChainID:            <CHAIN_ID>,
	}

	// Create a client
	client, _ := zerodev.NewClient(&clientConfig)
	defer client.Close()

	// Prepare call data
	zeroAddress := common.HexToAddress(zerodev.AddressZero)
	encodedCall, _ := zerodev.EncodeExecuteCall(&ethereum.CallMsg{
		To:    &zeroAddress,
		Value: big.NewInt(0),
		Data:  common.FromHex("0x"),
	})

	// Execute the call as user operation
	result, _ := client.SendUserOperation(walletAddress, encodedCall)
    
	// Get transaction hash
	fmt.Println(hexutil.Encode(*result.TxHash))
}
```
