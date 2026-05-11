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
//  3. We send a couple of plain regular-mode UserOps from the fleet EOA
//     to prove the fleet can act on the account without any further
//     coordination.
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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

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
		accountsURL  = flag.String("accounts-url", "https://accounts.dimo.org", "DIMO accounts service base URL")
		email        = flag.String("email", "", "email for the new shared user account (must be unique)")
		fleetPKHex   = flag.String("fleet-pk", "", "hex-encoded fleet EOA private key")
		rpcURLStr    = flag.String("rpc-url", "", "node RPC URL for chain reads (eth_getCode, eth_call)")
		bundlerURL   = flag.String("bundler-url", "", "ZeroDev bundler RPC URL (e.g. rpc.zerodev.app/api/v3/<id>/chain/<chainId>)")
		paymasterURL = flag.String("paymaster-url", "", "ZeroDev paymaster RPC URL (defaults to bundler-url)")
		chainID      = flag.Int64("chain-id", 137, "EVM chain id the kernel lives on")
	)
	flag.Parse()

	if *email == "" || *fleetPKHex == "" || *rpcURLStr == "" || *bundlerURL == "" {
		flag.Usage()
		return fmt.Errorf("missing required flag")
	}
	if *paymasterURL == "" {
		*paymasterURL = *bundlerURL
	}

	fleetPK, err := crypto.HexToECDSA(strings.TrimPrefix(*fleetPKHex, "0x"))
	if err != nil {
		return fmt.Errorf("parse fleet pk: %w", err)
	}
	fleetAddr := crypto.PubkeyToAddress(fleetPK.PublicKey)
	fmt.Printf("== fleet EOA: %s\n", fleetAddr.Hex())

	// 1) Register the user via accounts. By the time this returns, the
	//    kernel is on chain and the fleet's validator is installed.
	kernel, err := registerSharedAccount(*accountsURL, *email, fleetAddr)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	// 4) Two noop UserOps from the fleet, with different customNonceKeys
	//    so they don't queue on the same sequence stream.
	noop := &ethereum.CallMsg{
		To:    addrPtr(common.Address{}),
		Value: big.NewInt(0),
		Data:  []byte{},
	}

	fmt.Println("== fleet sends noop #1...")
	res1, err := client.SendCall(ctx, kernel, fleetPK, noop, 0x0001, true)
	reportResult("noop #1", res1, err)
	if err != nil && res1 == nil {
		return fmt.Errorf("SendCall #1: %w", err)
	}

	fmt.Println("== fleet sends noop #2...")
	res2, err := client.SendCall(ctx, kernel, fleetPK, noop, 0x0002, true)
	reportResult("noop #2", res2, err)
	if err != nil && res2 == nil {
		return fmt.Errorf("SendCall #2: %w", err)
	}

	fmt.Println("== done")
	return nil
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
// the kernel address accounts provisioned.
func registerSharedAccount(baseURL, email string, fleet common.Address) (common.Address, error) {
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
