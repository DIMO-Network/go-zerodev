// Package fleet provides a slim client for acting as the regular signer on
// a Kernel V3.1 shared account that was already provisioned + installed by
// an admin service (e.g. DIMO accounts at POST /api/shared/account/email).
//
// The admin handled all the on-chain setup: deployed the kernel, installed
// a weighted-ECDSA secondary validator with the admin's Turnkey EOA and
// the fleet's EOA as guardians (each at weight 100, threshold 100). By
// the time the fleet has the kernel address, the validator is already
// authorized for executeUserOp.
//
// What this package does for the fleet:
//   - SendCall: build, sign (EIP-191 personal-sign over userOpHash), and
//     submit a plain regular-mode UserOp through a ZeroDev bundler +
//     paymaster.
//   - IsFleetInstalled: defensive probe — eth_getCode + isAllowedSelector
//     read against the kernel. Useful for sanity-checking that the kernel
//     handed to us by the admin really is set up the way we expect before
//     we start sending UserOps.
//
// It deliberately does NOT include the enable-mode install path (deploy +
// install + first action in one UserOp) — the admin owns that step, and
// every entry point removed from this package was specific to that flow.
// See zdexplore/scripts 10–12 for the install-from-admin proof-of-life and
// the accounts service for the production implementation.
package fleet

import "github.com/ethereum/go-ethereum/common"

// Validator mode — byte 0 of the 24-byte Kernel V3 nonce key. We only
// ever submit DEFAULT-mode UserOps; ENABLE mode is exclusively the
// admin's concern.
const ValidatorModeDefault byte = 0x00

// Validator type — byte 1 of the nonce key. Mirrors the type byte that
// prefixes a 21-byte ValidationId stored on chain. SECONDARY for a
// weighted-ECDSA validator.
const ValidatorTypeSecondary byte = 0x01

// Kernel V3.1 known addresses. Deterministic CREATE2 deployments — same
// on every chain ZeroDev supports — but verify with eth_getCode on the
// target chain before relying on this (Polygon Amoy notably lacks the
// weighted-ECDSA validator at the time of writing).
var (
	EntryPoint07Address   = common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032")
	WeightedEcdsaAddress  = common.HexToAddress("0xeD89244160CfE273800B58b1B534031699dFeEEE")
)

// ExecuteUserOpSelector is the Kernel V3 selector that an EntryPoint v0.7
// UserOp's callData targets when the kernel implements the executeUserOp
// optimization. IsFleetInstalled checks that the regular validator has
// been granted permission for this selector specifically.
var ExecuteUserOpSelector = [4]byte{0xe9, 0xae, 0x5c, 0x53}
