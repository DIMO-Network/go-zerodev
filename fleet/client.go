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

// EnableArtifact is what the admin (e.g. DIMO accounts service) hands to
// the fleet. Pure data — no RPC interaction needed to produce it on the
// admin side beyond a single Turnkey typed-data signing call.
type EnableArtifact struct {
	KernelAddress     common.Address
	UserSignerAddress common.Address // EOA address of the kernel's sudo signer
	EnableSignature   []byte         // 65 bytes; user's Turnkey sig over the Enable typed-data hash
	FleetSignerAddr   common.Address // EOA address embedded in the enable signature (the validator's lone signer)
	KernelIndex       *big.Int       // CREATE2 salt; nil → 0
}

// ClientConfig wires the RPC + chain bits the fleet client needs.
type ClientConfig struct {
	RpcURL       *url.URL
	PaymasterURL *url.URL
	BundlerURL   *url.URL
	ChainID      *big.Int

	ReceiptPollingDelaySeconds int
	ReceiptPollingRetries      int
}

// Client sends fleet-side UserOps against a Kernel V3.1 with the
// weighted-ECDSA validator installed (or, on its first call against a
// given kernel, installs it via enable mode).
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

// SendCall sends a plain default-mode UserOp. Use when IsFleetInstalled
// has returned true — the weighted-ECDSA validator has already been
// installed and granted permission for executeUserOp.
func (c *Client) SendCall(
	ctx context.Context,
	kernel common.Address,
	fleetPK *ecdsa.PrivateKey,
	msg *ethereum.CallMsg,
	customNonceKey uint16,
	waitForReceipt bool,
) (*zerodev.UserOperationResult, error) {
	callData, err := zerodev.EncodeExecuteCall(msg)
	if err != nil {
		return nil, err
	}

	op, err := c.buildBaseUserOp(kernel, *callData, ValidatorModeDefault, customNonceKey)
	if err != nil {
		return nil, err
	}

	// Default-mode stub signature is just the 65-byte dummy.
	op.Signature = common.FromHex(zerodev.SignatureDummy)

	if err := c.sponsor(op); err != nil {
		return nil, err
	}

	if err := c.signAndAttach(op, fleetPK, nil); err != nil {
		return nil, err
	}

	return c.submit(op, waitForReceipt)
}

// SendInstallAndCall sends an enable-mode UserOp that installs the
// weighted-ECDSA validator (with selector permission) and then executes
// msg, all under one sudo-signed authorization (the artifact). If the
// kernel is not yet deployed this UserOp also runs the factory.
func (c *Client) SendInstallAndCall(
	ctx context.Context,
	art EnableArtifact,
	fleetPK *ecdsa.PrivateKey,
	msg *ethereum.CallMsg,
	customNonceKey uint16,
	waitForReceipt bool,
) (*zerodev.UserOperationResult, error) {
	if len(art.EnableSignature) == 0 {
		return nil, errors.New("fleet: empty enable signature on artifact")
	}

	cfg := WeightedEcdsaConfig{
		Signers:   []common.Address{art.FleetSignerAddr},
		Weights:   []uint32{100},
		Threshold: 100,
	}
	validatorData, err := cfg.EncodeInstallData()
	if err != nil {
		return nil, errors.Wrap(err, "fleet: install data")
	}
	sel, err := SelectorData(DefaultAction())
	if err != nil {
		return nil, errors.Wrap(err, "fleet: selector data")
	}

	callData, err := zerodev.EncodeExecuteCall(msg)
	if err != nil {
		return nil, err
	}

	op, err := c.buildBaseUserOp(art.KernelAddress, *callData, ValidatorModeEnable, customNonceKey)
	if err != nil {
		return nil, err
	}

	// If kernel isn't deployed yet, attach factory bits so the EntryPoint
	// deploys it in the same UserOp.
	code, err := getCode(ctx, c.rpc, art.KernelAddress)
	if err != nil {
		return nil, errors.Wrap(err, "fleet: eth_getCode")
	}
	if len(code) == 0 {
		fd, err := FactoryData(art.UserSignerAddress, art.KernelIndex)
		if err != nil {
			return nil, errors.Wrap(err, "fleet: factory data")
		}
		op.Factory = MetaFactoryAddress.Bytes()
		op.FactoryData = fd
	}

	// Build a stub envelope so the paymaster simulation can decode it.
	// The inner userOpSig is the standard ZeroDev dummy.
	stubEnv := EnableEnvelope{
		ValidatorData: validatorData,
		HookData:      []byte{},
		SelectorData:  sel,
		EnableSig:     art.EnableSignature,
		UserOpSig:     common.FromHex(zerodev.SignatureDummy),
	}
	stubBytes, err := stubEnv.Encode()
	if err != nil {
		return nil, err
	}
	op.Signature = stubBytes

	if err := c.sponsor(op); err != nil {
		return nil, err
	}

	if err := c.signAndAttach(op, fleetPK, &EnableEnvelope{
		ValidatorData: validatorData,
		HookData:      []byte{},
		SelectorData:  sel,
		EnableSig:     art.EnableSignature,
	}); err != nil {
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
	mode byte,
	customKey uint16,
) (*zerodev.UserOperation, error) {
	op := &zerodev.UserOperation{
		Sender:   kernel,
		CallData: callData,
	}

	// Nonce key + on-chain sequence.
	key := NonceKey(mode, ValidatorTypeSecondary, WeightedEcdsaAddress, customKey)
	seq, err := c.entryNonceForKey(kernel, key)
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

// entryNonceForKey wraps EntryPoint.getNonce(account, key) with our
// 24-byte key encoding. The existing zerodev.Entrypoint07.GetNonce uses a
// different (sudo-friendly) key encoding, so we call the custom-key form.
func (c *Client) entryNonceForKey(account common.Address, key [24]byte) (uint64, error) {
	return c.entry.GetNonceWithKey(account, NonceKeyAsUint192(key))
}

// sponsor calls the ZeroDev paymaster RPC with the stub signature the
// caller has already attached to op.Signature, and fills the gas +
// paymaster fields on the op from the response.
func (c *Client) sponsor(op *zerodev.UserOperation) error {
	resp, err := c.paymaster.SponsorUserOperationWithStub(op)
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
// the EIP-191 personal-message wrapper the weighted-ECDSA validator
// expects, and assembles the final signature. envOrNil controls mode: nil
// means default mode (raw 65-byte sig); non-nil means enable mode (envelope
// wrapping the userOpSig).
func (c *Client) signAndAttach(op *zerodev.UserOperation, pk *ecdsa.PrivateKey, envOrNil *EnableEnvelope) error {
	hash, err := c.entry.GetUserOperationHash(op)
	if err != nil {
		return err
	}
	sig, err := PersonalSignUserOpHash(pk, *hash)
	if err != nil {
		return err
	}
	if envOrNil == nil {
		op.Signature = sig
		return nil
	}
	envOrNil.UserOpSig = sig
	final, err := envOrNil.Encode()
	if err != nil {
		return err
	}
	op.Signature = final
	return nil
}

// submit sends a built+signed UserOp through the bundler and, when
// waitForReceipt is true, polls for the receipt. The contract here:
//
//   - If the bundler rejects the UserOp, returns (nil, error). The UserOp
//     didn't land.
//   - If the bundler accepts it but receipt polling fails (timeout, RPC
//     error, malformed response), returns a non-nil result with the
//     userOpHash populated AND a non-nil error. The caller has enough to
//     investigate manually (look up the hash on the explorer) but knows
//     the wait part didn't complete.
//   - On full success, returns (result, nil) with receipt populated.
//
// The previous version swallowed the receipt-polling error, which made
// "I sent a UserOp but the receipt never came back" indistinguishable from
// "everything worked" — exactly the wrong tradeoff.
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
