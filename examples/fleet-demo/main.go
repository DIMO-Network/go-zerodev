// fleet-demo: end-to-end exercise of the variant-B shared-account flow.
//
//  1. POST { email, providedSignerAddress } to DIMO accounts. The service
//     creates the Turnkey sub-org, deploys the kernel, and installs a
//     weighted-ECDSA secondary validator with the Turnkey EOA and the
//     fleet EOA as co-guardians (each weight 100, threshold 100). All in
//     one Turnkey-signed enable-mode UserOp on the server side. Returns
//     { walletAddress } — the kernel address.
//
//  2. We probe the kernel with IsFleetInstalled to confirm it really is
//     set up the way accounts claims.
//
//  3. The fleet signs a UserOp that has the kernel call
//     DIMORegistry.mintVehicleWithDeviceDefinition. We parse the
//     resulting receipt to recover the freshly minted vehicle tokenId
//     from the ERC-721 Transfer event on the VehicleId NFT.
//
//  4. The fleet signs a second UserOp that has the kernel
//     safeTransferFrom the new tokenId to a throwaway EOA generated at
//     startup. VehicleId._transfer requires msg.sender == from, so the
//     kernel must drive the transfer itself — exactly what the fleet
//     authority enables.
//
// The fleet PK does NOT need to hold any funds — the ZeroDev paymaster
// covers gas. The same email cannot be reused: accounts rejects
// "User already exists." Generate a fresh email per run.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DIMO-Network/go-zerodev"
	"github.com/DIMO-Network/go-zerodev/fleet"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

const registryMintABI = `[{
    "type":"function",
    "name":"mintVehicleWithDeviceDefinition",
    "stateMutability":"nonpayable",
    "inputs":[
        {"name":"manufacturerNode","type":"uint256"},
        {"name":"owner","type":"address"},
        {"name":"storageNodeId","type":"uint256"},
        {"name":"deviceDefinitionId","type":"string"},
        {"name":"attrInfo","type":"tuple[]","components":[
            {"name":"attribute","type":"string"},
            {"name":"info","type":"string"}
        ]}
    ],
    "outputs":[]
}]`

const vehicleIDTransferABI = `[{
    "type":"function",
    "name":"safeTransferFrom",
    "stateMutability":"nonpayable",
    "inputs":[
        {"name":"from","type":"address"},
        {"name":"to","type":"address"},
        {"name":"tokenId","type":"uint256"}
    ],
    "outputs":[]
}]`

// keccak256("Transfer(address,address,uint256)")
var erc721TransferTopic = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

// AttrInfoPair mirrors the AttributeInfoPair tuple in the registry ABI.
// Fields are matched to ABI components case-insensitively.
type AttrInfoPair struct {
	Attribute string
	Info      string
}

type sharedAccountRequest struct {
	Email                 string `json:"email"`
	ProvidedSignerAddress string `json:"providedSignerAddress"`
}

type sharedAccountResponse struct {
	WalletAddress string `json:"walletAddress"`
	Error         any    `json:"error,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("fleet-demo: %v", err)
	}
}

func run() error {
	var (
		accountsURL    = flag.String("accounts-url", "https://accounts.dimo.org", "DIMO accounts service base URL")
		accountsJWT    = flag.String("accounts-jwt", "", "bearer JWT for accounts; its ethereum_address claim must be in the SHARED_ACCOUNT_ALLOWED_JWT_CALLER_ADDRESSES allowlist")
		email          = flag.String("email", "", "email for the new shared user account (must be unique)")
		fleetPKHex     = flag.String("fleet-pk", "", "hex-encoded fleet EOA private key")
		rpcURLStr      = flag.String("rpc-url", "", "node RPC URL for chain reads (eth_getCode, eth_call)")
		bundlerURL     = flag.String("bundler-url", "", "ZeroDev bundler RPC URL (e.g. rpc.zerodev.app/api/v3/<id>/chain/<chainId>)")
		paymasterURL   = flag.String("paymaster-url", "", "ZeroDev paymaster RPC URL (defaults to bundler-url)")
		chainID        = flag.Int64("chain-id", 137, "EVM chain id the kernel lives on")
		registryAddr   = flag.String("registry-addr", "0xFA8beC73cebB9D88FF88a2f75E7D7312f2Fd39EC", "DIMORegistry proxy address")
		vehicleIDAddr  = flag.String("vehicle-id-addr", "0xbA5738a18d83D41847dfFbDC6101d37C69c9B0cF", "VehicleId NFT proxy address")
		mfrNodeStr     = flag.String("manufacturer-node", "", "manufacturer node id (decimal uint256)")
		ddID           = flag.String("device-definition-id", "", "device definition id (slug)")
		storageNodeStr = flag.String("storage-node-id", "0", "storage node id (decimal uint256)")
	)
	flag.Parse()

	if *email == "" || *fleetPKHex == "" || *rpcURLStr == "" || *bundlerURL == "" ||
		*mfrNodeStr == "" || *ddID == "" || *accountsJWT == "" {
		flag.Usage()
		return fmt.Errorf("missing required flag")
	}
	if *paymasterURL == "" {
		*paymasterURL = *bundlerURL
	}

	manufacturerNode, ok := new(big.Int).SetString(*mfrNodeStr, 10)
	if !ok {
		return fmt.Errorf("manufacturer-node: not a decimal uint256: %q", *mfrNodeStr)
	}
	storageNodeID, ok := new(big.Int).SetString(*storageNodeStr, 10)
	if !ok {
		return fmt.Errorf("storage-node-id: not a decimal uint256: %q", *storageNodeStr)
	}
	registry := common.HexToAddress(*registryAddr)
	vehicleID := common.HexToAddress(*vehicleIDAddr)

	fleetPK, err := crypto.HexToECDSA(strings.TrimPrefix(*fleetPKHex, "0x"))
	if err != nil {
		return fmt.Errorf("parse fleet pk: %w", err)
	}
	fleetAddr := crypto.PubkeyToAddress(fleetPK.PublicKey)
	fmt.Printf("== fleet EOA: %s\n", fleetAddr.Hex())

	// 1) Register the user via accounts. By the time this returns, the
	//    kernel is on chain and the fleet's validator is installed.
	kernel, err := registerSharedAccount(*accountsURL, *accountsJWT, *email, fleetAddr)
	if err != nil {
		return fmt.Errorf("register shared account: %w", err)
	}
	fmt.Printf("== accounts returned kernel %s\n", kernel.Hex())

	// 2) Wire up the fleet client.
	rpcU, err := url.Parse(*rpcURLStr)
	if err != nil {
		return fmt.Errorf("rpc-url: %w", err)
	}
	bundlerU, err := url.Parse(*bundlerURL)
	if err != nil {
		return fmt.Errorf("bundler-url: %w", err)
	}
	paymasterU, err := url.Parse(*paymasterURL)
	if err != nil {
		return fmt.Errorf("paymaster-url: %w", err)
	}

	client, err := fleet.NewClient(&fleet.ClientConfig{
		RpcURL:                     rpcU,
		BundlerURL:                 bundlerU,
		PaymasterURL:               paymasterU,
		ChainID:                    big.NewInt(*chainID),
		ReceiptPollingDelaySeconds: 2,
		ReceiptPollingRetries:      60,
	})
	if err != nil {
		return fmt.Errorf("fleet client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 3) Sanity: the kernel should already have the fleet's validator
	//    installed because accounts did it.
	installed, err := client.IsFleetInstalled(ctx, kernel)
	if err != nil {
		return fmt.Errorf("IsFleetInstalled: %w", err)
	}
	if !installed {
		return fmt.Errorf("kernel %s claims to be provisioned but isAllowedSelector returned false — something went wrong on the accounts side", kernel.Hex())
	}
	fmt.Println("== installed: true (expected — accounts did the install)")

	// 4) Throwaway recipient for the safe-transfer step.
	recipientPK, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate recipient key: %w", err)
	}
	recipient := crypto.PubkeyToAddress(recipientPK.PublicKey)
	fmt.Printf("== generated recipient EOA: %s\n", recipient.Hex())
	fmt.Printf("   recipient PK: %s\n", hexutil.Encode(crypto.FromECDSA(recipientPK)))

	// 5) Mint a vehicle to the kernel.
	mintData, err := packMint(manufacturerNode, kernel, storageNodeID, *ddID)
	if err != nil {
		return fmt.Errorf("pack mint: %w", err)
	}
	mintMsg := &ethereum.CallMsg{
		To:    addrPtr(registry),
		Value: big.NewInt(0),
		Data:  mintData,
	}

	fmt.Println("== fleet sends mintVehicleWithDeviceDefinition...")
	mintRes, err := client.SendCall(ctx, kernel, fleetPK, mintMsg, 0x0001, true)
	reportResult("mint", mintRes, err)
	if err != nil {
		return fmt.Errorf("mint SendCall: %w", err)
	}

	tokenID, err := extractMintedTokenID(mintRes.Receipt, vehicleID, kernel)
	if err != nil {
		return fmt.Errorf("recover tokenId: %w", err)
	}
	fmt.Printf("== minted tokenId = %s\n", tokenID.String())

	// 6) Safe-transfer it to the throwaway EOA.
	transferData, err := packSafeTransferFrom(kernel, recipient, tokenID)
	if err != nil {
		return fmt.Errorf("pack safeTransferFrom: %w", err)
	}
	transferMsg := &ethereum.CallMsg{
		To:    addrPtr(vehicleID),
		Value: big.NewInt(0),
		Data:  transferData,
	}

	fmt.Println("== fleet sends safeTransferFrom...")
	transferRes, err := client.SendCall(ctx, kernel, fleetPK, transferMsg, 0x0001, true)
	reportResult("transfer", transferRes, err)
	if err != nil {
		return fmt.Errorf("transfer SendCall: %w", err)
	}

	fmt.Println("== done")
	return nil
}

// packMint ABI-encodes a call to
// mintVehicleWithDeviceDefinition(uint256,address,uint256,string,
// (string,string)[]) with an empty attrInfo array.
func packMint(manufacturerNode *big.Int, owner common.Address, storageNodeID *big.Int, ddID string) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(registryMintABI))
	if err != nil {
		return nil, err
	}
	return parsed.Pack("mintVehicleWithDeviceDefinition",
		manufacturerNode,
		owner,
		storageNodeID,
		ddID,
		[]AttrInfoPair{},
	)
}

// packSafeTransferFrom ABI-encodes a call to ERC-721
// safeTransferFrom(address,address,uint256).
func packSafeTransferFrom(from, to common.Address, tokenID *big.Int) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(vehicleIDTransferABI))
	if err != nil {
		return nil, err
	}
	return parsed.Pack("safeTransferFrom", from, to, tokenID)
}

// extractMintedTokenID walks the inner logs from a UserOp receipt and
// returns the tokenId of the ERC-721 Transfer event on vehicleIDAddr
// whose `from` is the zero address (mints) and whose `to` is the
// kernel. Returns an error if no such log is found — that means the
// mint didn't actually happen even though receipt polling succeeded.
func extractMintedTokenID(receipt *zerodev.UserOperationReceipt, vehicleIDAddr, kernel common.Address) (*big.Int, error) {
	if receipt == nil {
		return nil, fmt.Errorf("mint receipt is nil")
	}
	kernelTopic := common.BytesToHash(kernel.Bytes())
	for _, lg := range receipt.Logs {
		if lg.Address != vehicleIDAddr {
			continue
		}
		if len(lg.Topics) != 4 || lg.Topics[0] != erc721TransferTopic {
			continue
		}
		if lg.Topics[1] != (common.Hash{}) {
			continue
		}
		if lg.Topics[2] != kernelTopic {
			continue
		}
		return new(big.Int).SetBytes(lg.Topics[3].Bytes()), nil
	}
	return nil, fmt.Errorf("no mint Transfer log found on %s with to=%s", vehicleIDAddr.Hex(), kernel.Hex())
}

// reportResult prints what we know about a UserOp submission, including
// the userOpHash (present when the bundler accepted), the L1 transaction
// hash (present when receipt polling succeeded), and any receipt-polling
// error (UserOp may still have landed — look up the hash).
func reportResult(label string, res *zerodev.UserOperationResult, err error) {
	if res == nil {
		fmt.Printf("   %s: send failed: %v\n", label, err)
		return
	}
	fmt.Printf("   userOpHash = %s\n", hexutil.Encode(res.UserOperationHash))
	if err != nil {
		fmt.Printf("   WARNING: %v\n", err)
		fmt.Printf("   (the UserOp may still have landed; check the userOpHash on the explorer)\n")
		return
	}
	if res.Receipt == nil {
		fmt.Printf("   receipt = <nil> (waitForReceipt was false)\n")
		return
	}
	if res.Receipt.TransactionHash != nil {
		fmt.Printf("   txHash     = %s\n", res.Receipt.TransactionHash.String())
		return
	}
	raw, _ := json.MarshalIndent(res.Receipt, "   ", "  ")
	fmt.Printf("   receipt (no txHash field):\n   %s\n", raw)
}

// registerSharedAccount calls POST /api/shared/account/email and returns
// the kernel address accounts provisioned. The JWT's ethereum_address
// claim must be in the accounts service's caller allowlist.
func registerSharedAccount(baseURL, jwt, email string, fleet common.Address) (common.Address, error) {
	body, _ := json.Marshal(sharedAccountRequest{
		Email:                 email,
		ProvidedSignerAddress: fleet.Hex(),
	})

	endpoint := strings.TrimRight(baseURL, "/") + "/api/shared/account/email"
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return common.Address{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+jwt)

	client := &http.Client{Timeout: 120 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return common.Address{}, err
	}
	defer httpResp.Body.Close()

	raw, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode/100 != 2 {
		return common.Address{}, fmt.Errorf("accounts %d: %s", httpResp.StatusCode, string(raw))
	}

	var resp sharedAccountResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return common.Address{}, fmt.Errorf("decode accounts response: %w (raw: %s)", err, raw)
	}
	if resp.WalletAddress == "" {
		return common.Address{}, fmt.Errorf("accounts returned empty walletAddress: %s", raw)
	}
	return common.HexToAddress(resp.WalletAddress), nil
}

func addrPtr(a common.Address) *common.Address { return &a }
