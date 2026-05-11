// fleet-demo: end-to-end exercise of the fleet flow against accounts.dimo.org.
//
// Steps it runs, in order:
//
//  1. POST { email, providedSignerAddress } to the DIMO accounts service.
//     The service creates the Turnkey sub-org, deploys the kernel sudo-only,
//     and returns { walletAddress (= kernel address), enableSignature }.
//
//  2. With the artifact + the fleet PK, build a fleet.Client and submit one
//     "install + execute" UserOp via the ZeroDev bundler. This installs the
//     weighted-ECDSA validator backed by the fleet key as a regular
//     validator on the kernel, with selector permission for executeUserOp,
//     and runs a no-op call all in the same UserOp. After this lands, the
//     fleet has standing authority over the kernel.
//
//  3. Submit a second, plain-regular-mode UserOp to demonstrate that
//     follow-on operations don't need the enable envelope.
//
// Usage:
//
//	go run ./examples/fleet-demo \
//	    -accounts-url=https://accounts.dimo.org \
//	    -email=fleet-demo-$(date +%s)@example.com \
//	    -fleet-pk=0x... \
//	    -rpc-url=https://polygon-rpc.com \
//	    -bundler-url='https://rpc.zerodev.app/api/v3/<projectId>/chain/137' \
//	    -chain-id=137
//
// The fleet PK does NOT need to hold any funds — the ZeroDev paymaster
// covers gas. The same email cannot be reused: the accounts service
// rejects "User already exists." Generate a fresh email per run.
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
	"os"
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
	// WalletAddress is the kernel (smart-account) address. Despite the
	// name; the API field is named for back-compat. See the rename
	// discussion / DIMO-Network/accounts PR for context.
	WalletAddress string `json:"walletAddress"`
	// SudoSignerAddress is the Turnkey EOA that owns the kernel's root
	// validator. Added in DIMO-Network/accounts#61. Optional in the
	// decode so this binary still parses responses from an accounts
	// service that hasn't shipped the change yet — but if it's missing
	// and the kernel isn't already deployed, the install-carrying UserOp
	// will fail because we can't build the right factoryData without the
	// sudo signer.
	SudoSignerAddress string `json:"sudoSignerAddress"`
	EnableSignature   string `json:"enableSignature"`
	Error             any    `json:"error,omitempty"`
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
		chainID      = flag.Int64("chain-id", 137, "EVM chain id the artifact targets")
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

	// 1) Ask accounts.dimo.org for the artifact.
	art, err := registerSharedAccount(*accountsURL, *email, fleetAddr)
	if err != nil {
		return fmt.Errorf("register shared account: %w", err)
	}
	fmt.Printf("== accounts returned:\n   kernelAddress     = %s\n   sudoSignerAddress = %s\n   enableSignature   = %s\n",
		art.KernelAddress.Hex(), art.UserSignerAddress.Hex(), hexutil.Encode(art.EnableSignature))
	if art.UserSignerAddress == (common.Address{}) {
		fmt.Println("   WARNING: sudoSignerAddress missing from response. This means the accounts service")
		fmt.Println("   hasn't shipped DIMO-Network/accounts#61 yet. If the kernel is already deployed on")
		fmt.Println("   chain (older accounts versions deployed sudo-only as part of the request) the demo")
		fmt.Println("   still works because the factory branch is skipped. If the kernel is undeployed,")
		fmt.Println("   SendInstallAndCall will build factoryData with a zero sudo signer and the UserOp")
		fmt.Println("   will fail with a sender-address mismatch.")
	}

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
		RpcURL:       rpcU,
		BundlerURL:   bundlerU,
		PaymasterURL: paymasterU,
		ChainID:      big.NewInt(*chainID),
		// Demo wants quick feedback. The library default is 10s × 24
		// retries (~4 min), which is fine for batch jobs but feels dead
		// for an interactive run.
		ReceiptPollingDelaySeconds: 2,
		ReceiptPollingRetries:      60,
	})
	if err != nil {
		return fmt.Errorf("fleet client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Sanity: not installed yet (accounts only deploys sudo, doesn't install
	// the secondary validator).
	installed, err := client.IsFleetInstalled(ctx, art.KernelAddress)
	if err != nil {
		return fmt.Errorf("IsFleetInstalled (pre): %w", err)
	}
	fmt.Printf("== installed before: %v (expect false)\n", installed)

	// 3) Install-carrying UserOp: install regular validator + run a noop.
	noop := &ethereum.CallMsg{
		To:    addrPtr(common.Address{}),
		Value: big.NewInt(0),
		Data:  []byte{},
	}

	fmt.Println("== sending install-and-call UserOp...")
	res, err := client.SendInstallAndCall(ctx, art, fleetPK, noop, 0x0001 /* customNonceKey */, true)
	reportResult("install-and-call", res, err)
	if err != nil && res == nil {
		return fmt.Errorf("SendInstallAndCall: %w", err)
	}

	// 4) Confirm installation is now visible on chain.
	installed, err = client.IsFleetInstalled(ctx, art.KernelAddress)
	if err != nil {
		return fmt.Errorf("IsFleetInstalled (post): %w", err)
	}
	fmt.Printf("== installed after: %v (expect true)\n", installed)
	if !installed {
		return fmt.Errorf("install did not take effect")
	}

	// 5) Follow-on plain regular-mode UserOp.
	fmt.Println("== sending plain regular-mode UserOp...")
	res2, err := client.SendCall(ctx, art.KernelAddress, fleetPK, noop, 0x0002, true)
	reportResult("regular-mode", res2, err)
	if err != nil && res2 == nil {
		return fmt.Errorf("SendCall: %w", err)
	}

	fmt.Println("== done")
	return nil
}

// reportResult prints what we know about a UserOp submission, including the
// userOpHash (always present when the bundler accepted the UserOp), the
// L1 transaction hash (when receipt polling succeeded), and any error from
// receipt polling (UserOp may still have landed — go look up the hash).
func reportResult(label string, res *zerodev.UserOperationResult, err error) {
	if res == nil {
		// Send itself failed; the caller will see the error and abort.
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
	} else {
		// Receipt came back but transactionHash is missing — that's odd
		// enough to be worth dumping for the next debugging session.
		raw, _ := json.MarshalIndent(res.Receipt, "   ", "  ")
		fmt.Printf("   receipt (no txHash field):\n   %s\n", raw)
	}
}

// registerSharedAccount calls POST /api/shared/account/email and returns
// the artifact the fleet needs.
func registerSharedAccount(baseURL, email string, fleet common.Address) (fleetArtifactWithSig, error) {
	body, _ := json.Marshal(sharedAccountRequest{
		Email:                 email,
		ProvidedSignerAddress: fleet.Hex(),
	})

	endpoint := strings.TrimRight(baseURL, "/") + "/api/shared/account/email"
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fleetArtifactWithSig{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return fleetArtifactWithSig{}, err
	}
	defer httpResp.Body.Close()

	raw, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode/100 != 2 {
		return fleetArtifactWithSig{}, fmt.Errorf("accounts %d: %s", httpResp.StatusCode, string(raw))
	}

	var resp sharedAccountResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fleetArtifactWithSig{}, fmt.Errorf("decode accounts response: %w (raw: %s)", err, raw)
	}
	if resp.WalletAddress == "" || resp.EnableSignature == "" {
		return fleetArtifactWithSig{}, fmt.Errorf("accounts returned empty artifact: %s", raw)
	}

	sigBytes, err := hexutil.Decode(resp.EnableSignature)
	if err != nil {
		return fleetArtifactWithSig{}, fmt.Errorf("decode enableSignature: %w", err)
	}

	// sudoSignerAddress may be empty against an accounts service that
	// hasn't shipped DIMO-Network/accounts#61. The caller (run() above)
	// handles that case and warns the user.
	var sudo common.Address
	if resp.SudoSignerAddress != "" {
		sudo = common.HexToAddress(resp.SudoSignerAddress)
	}

	return fleetArtifactWithSig{
		KernelAddress:     common.HexToAddress(resp.WalletAddress),
		UserSignerAddress: sudo,
		EnableSignature:   sigBytes,
		FleetSignerAddr:   fleet,
	}, nil
}

// fleetArtifactWithSig is a constructor-friendly shorthand around
// fleet.EnableArtifact for this demo.
type fleetArtifactWithSig = fleet.EnableArtifact

func addrPtr(a common.Address) *common.Address { return &a }

// Ensure imports stay referenced even if a future edit drops a use.
var _ = os.Stdout
var _ = zerodev.SignatureDummy
