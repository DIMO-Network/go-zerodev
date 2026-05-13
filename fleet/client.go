package fleet

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"net/url"

	"github.com/DIMO-Network/go-zerodev"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/friendsofgo/errors"
)

// ClientConfig wires the RPC + chain bits the fleet client needs.
type ClientConfig struct {
	RpcURL       *url.URL
	PaymasterURL *url.URL
	BundlerURL   *url.URL
	ChainID      *big.Int

	ReceiptPollingDelaySeconds int
	ReceiptPollingRetries      int
}

// Client sends regular-mode UserOps against a Kernel V3.1 shared account
// that has the weighted-ECDSA secondary validator already installed (with
// the caller's fleet EOA among the guardians).
type Client struct {
	chainID *big.Int

	rpc       *rpc.Client
	bundler   *zerodev.BundlerClient
	paymaster *zerodev.PaymasterClient
	entry     zerodev.Entrypoint

	pollDelay   int
	pollRetries int
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg.RpcURL == nil || cfg.BundlerURL == nil || cfg.PaymasterURL == nil || cfg.ChainID == nil {
		return nil, errors.New("fleet: rpcURL, bundlerURL, paymasterURL, chainID required")
	}

	netRpc, err := rpc.Dial(cfg.RpcURL.String())
	if err != nil {
		return nil, errors.Wrap(err, "fleet: rpc dial")
	}
	bundlerRpc, err := rpc.Dial(cfg.BundlerURL.String())
	if err != nil {
		netRpc.Close()
		return nil, errors.Wrap(err, "fleet: bundler dial")
	}
	paymasterRpc, err := rpc.Dial(cfg.PaymasterURL.String())
	if err != nil {
		netRpc.Close()
		bundlerRpc.Close()
		return nil, errors.Wrap(err, "fleet: paymaster dial")
	}

	entry, err := zerodev.NewEntrypoint07(netRpc, cfg.ChainID)
	if err != nil {
		netRpc.Close()
		bundlerRpc.Close()
		paymasterRpc.Close()
		return nil, err
	}
	bundler, err := zerodev.NewBundlerClient(bundlerRpc, entry, cfg.ChainID)
	if err != nil {
		netRpc.Close()
		bundlerRpc.Close()
		paymasterRpc.Close()
		return nil, err
	}
	paymaster, err := zerodev.NewPaymasterClient(paymasterRpc, entry, cfg.ChainID)
	if err != nil {
		netRpc.Close()
		bundlerRpc.Close()
		paymasterRpc.Close()
		return nil, err
	}

	delay := 10
	if cfg.ReceiptPollingDelaySeconds > 0 {
		delay = cfg.ReceiptPollingDelaySeconds
	}
	retries := 24
	if cfg.ReceiptPollingRetries > 0 {
		retries = cfg.ReceiptPollingRetries
	}

	return &Client{
		chainID:     cfg.ChainID,
		rpc:         netRpc,
		bundler:     bundler,
		paymaster:   paymaster,
		entry:       entry,
		pollDelay:   delay,
		pollRetries: retries,
	}, nil
}

func (c *Client) Close() {
	if c.rpc != nil {
		c.rpc.Close()
	}
}

// IsFleetInstalled wraps the package-level helper using this client's RPC.
func (c *Client) IsFleetInstalled(ctx context.Context, kernel common.Address) (bool, error) {
	return IsFleetInstalled(ctx, c.rpc, kernel)
}

// SendCall sends a plain regular-mode UserOp through the bundler. The
// kernel must already have the weighted-ECDSA secondary validator
// installed and granted permission for executeUserOp (which is what the
// admin's POST /api/shared/account/email leaves behind).
//
// The UserOp uses the default nonce lane (customNonceKey = 0) for the
// fleet's weighted-ECDSA validator. That lane is independent of the
// sudo validator's lane, so fleet and admin ops can run in parallel.
// Multiple fleet UserOps on the same kernel serialize within this lane.
func (c *Client) SendCall(
	ctx context.Context,
	kernel common.Address,
	fleetPK *ecdsa.PrivateKey,
	msg *ethereum.CallMsg,
	waitForReceipt bool,
) (*zerodev.UserOperationResult, error) {
	callData, err := zerodev.EncodeExecuteCall(msg)
	if err != nil {
		return nil, err
	}

	op, err := c.buildBaseUserOp(kernel, *callData)
	if err != nil {
		return nil, err
	}

	// paymaster.SponsorUserOperation overwrites op.Signature with the
	// standard 65-byte dummy stub — exactly what the weighted-ECDSA
	// validator's getStubSignature path expects for a single-signer
	// regular-mode UserOp, so we don't need a custom stub here.
	if err := c.sponsor(op); err != nil {
		return nil, err
	}
	if err := c.signAndAttach(op, fleetPK); err != nil {
		return nil, err
	}

	return c.submit(op, waitForReceipt)
}

// buildBaseUserOp fills in everything that doesn't depend on the paymaster
// response or the signature: sender, nonce, callData, and the gas-price
// fields the bundler suggests.
func (c *Client) buildBaseUserOp(
	kernel common.Address,
	callData []byte,
) (*zerodev.UserOperation, error) {
	op := &zerodev.UserOperation{
		Sender:   kernel,
		CallData: callData,
	}

	key := NonceKeyDefaultModeForWeightedEcdsa(0)
	seq, err := c.entry.GetNonceWithKey(kernel, NonceKeyAsUint192(key))
	if err != nil {
		return nil, err
	}
	op.Nonce = FullNonce(key, seq)

	gas, err := c.bundler.GetUserOperationGasPrice()
	if err != nil {
		return nil, err
	}
	op.MaxFeePerGas = gas.Standard.MaxFeePerGas
	op.MaxPriorityFeePerGas = gas.Standard.MaxPriorityFeePerGas

	return op, nil
}

// sponsor calls the ZeroDev paymaster RPC and fills the gas + paymaster
// fields on the op from the response.
func (c *Client) sponsor(op *zerodev.UserOperation) error {
	resp, err := c.paymaster.SponsorUserOperation(op)
	if err != nil {
		return err
	}
	op.CallGasLimit = resp.CallGasLimit
	op.VerificationGasLimit = resp.VerificationGasLimit
	op.PreVerificationGas = resp.PreVerificationGas
	op.PaymasterVerificationGasLimit = resp.PaymasterVerificationGasLimit
	op.PaymasterPostOpGasLimit = resp.PaymasterPostOpGasLimit
	op.Paymaster = resp.Paymaster
	op.PaymasterData = resp.PaymasterData
	return nil
}

// signAndAttach computes the userOpHash, signs it with the fleet's PK via
// EIP-191 personal-sign (what weighted-ECDSA expects from a single
// guardian), and writes the 65-byte signature into op.Signature.
func (c *Client) signAndAttach(op *zerodev.UserOperation, pk *ecdsa.PrivateKey) error {
	hash, err := c.entry.GetUserOperationHash(op)
	if err != nil {
		return err
	}
	sig, err := PersonalSignUserOpHash(pk, *hash)
	if err != nil {
		return err
	}
	op.Signature = sig
	return nil
}

// submit sends a built+signed UserOp through the bundler and, when
// waitForReceipt is true, polls for the receipt. Contract:
//
//   - bundler rejects: returns (nil, error). UserOp didn't land.
//   - bundler accepts, receipt poll fails: returns (result with
//     userOpHash, error). Landed but wait didn't complete; look up the
//     hash on an explorer.
//   - full success: returns (result with receipt, nil).
func (c *Client) submit(op *zerodev.UserOperation, waitForReceipt bool) (*zerodev.UserOperationResult, error) {
	hash, err := c.bundler.SendUserOperation(op)
	if err != nil {
		return nil, err
	}
	result := &zerodev.UserOperationResult{UserOperationHash: hash}
	if !waitForReceipt {
		return result, nil
	}
	receipt, rerr := c.bundler.GetUserOperationReceipt(hash, c.pollDelay, c.pollRetries)
	if rerr != nil {
		return result, errors.Wrap(rerr, "fleet: userOp sent but receipt fetch failed")
	}
	result.Receipt = receipt
	return result, nil
}
