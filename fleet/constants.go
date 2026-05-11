// Package fleet adds support for acting as a "regular" (secondary) validator
// on a Kernel V3.1 smart account that the holder of the fleet key did not
// deploy and does not own as root. The typical flow:
//
//   1. An admin service (e.g. DIMO accounts) has the user's Turnkey EOA
//      sign an EIP-712 Enable typed-data hash authorizing a specific fleet
//      EOA to act as a weighted-ECDSA secondary validator on the user's
//      Kernel V3.1 address. It returns an EnableArtifact.
//
//   2. The fleet's first UserOp against that kernel is an "enable mode"
//      UserOp that includes the artifact's enable signature plus the fleet's
//      own UserOp signature. The Kernel V3.1 protocol requires this — there
//      is no public function that lets sudo install a usable secondary
//      validator with the selector allow-list set, so the install has to
//      ride alongside the regular validator's first signed UserOp.
//
//   3. Every subsequent fleet UserOp is a plain "default mode" UserOp that
//      simply ECDSA-signs (with EIP-191 personal-message wrapping) the
//      EntryPoint userOpHash.
//
// The fleet package shares the lower-level RPC/encoding plumbing in the
// parent package (Entrypoint, BundlerClient, PaymasterClient, UserOperation)
// but supplies its own UserOp builder, nonce encoder, signature wrapper, and
// signer because the existing Client is opinionated around "I am the AA
// wallet owner."
package fleet

import "github.com/ethereum/go-ethereum/common"

// Validator mode bytes — top byte of the 24-byte Kernel V3 nonce key.
// DEFAULT for a plain regular-mode UserOp, ENABLE for the install-carrying
// first UserOp from a secondary validator.
const (
	ValidatorModeDefault byte = 0x00
	ValidatorModeEnable  byte = 0x01
)

// Validator type bytes — second byte of the nonce key. Mirrors the type
// byte that prefixes a 21-byte ValidationId stored on chain.
const (
	ValidatorTypeSudo      byte = 0x00
	ValidatorTypeSecondary byte = 0x01
)

// Kernel V3 CALL_TYPE — used inside the selectorData blob that's part of
// both the Enable typed-data hash AND the signature envelope. DELEGATE_CALL
// here is "the call type the kernel will use when invoking the selector's
// configured target." We don't actually delegate-call anything (target=0),
// but the value must match what the SDK's getPluginsEnableTypedData produces
// or the EIP-712 hash diverges and the sudo signature fails to recover.
var CallTypeDelegateCall = []byte{0xff}

// Kernel V3.1 known addresses. The factory + entry point + validator
// contracts use deterministic CREATE2 deployments and are at the same
// address on every chain ZeroDev supports — verify with eth_getCode on the
// target chain before deploying anything new (Polygon Amoy notably is
// missing WeightedEcdsaValidator).
var (
	EntryPoint07Address    = common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
	MetaFactoryAddress     = common.HexToAddress("0xd703aaE79538628d27099B8c4f621bE4CCd142d5")
	KernelFactoryAddress   = common.HexToAddress("0xaac5D4240AF87249B3f71BC8E4A2cae074A3E419")
	EcdsaValidatorAddress  = common.HexToAddress("0x845ADb2C711129d4f3966735eD98a9F09fC4cE57")
	WeightedEcdsaAddress   = common.HexToAddress("0xeD89244160CfE273800B58b1B534031699dFeEEE")
)

// KernelVersion is the EIP-712 domain version string the kernel reports
// after deployment. For an undeployed kernel we hardcode the same value so
// the typed-data hash we sign matches what the deployed contract will
// produce.
const KernelVersion = "0.3.1"

// KernelName is the EIP-712 domain name.
const KernelName = "Kernel"

// ExecuteUserOpSelector is the Kernel V3 selector that an EntryPoint v0.7
// UserOp's callData targets when the kernel implements the executeUserOp
// optimization. The selector allow-list (isAllowedSelector) on the kernel
// must contain this selector for the secondary validator before plain
// regular-mode UserOps can be validated.
var ExecuteUserOpSelector = [4]byte{0xe9, 0xae, 0x5c, 0x53}
