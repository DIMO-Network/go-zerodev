package fleet

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// EnableTypedData builds the EIP-712 typed-data hash the user's sudo signer
// must sign to authorize installing a regular validator. The accounts
// service (or any admin) computes this hash, asks Turnkey to sign it, and
// ships the resulting 65-byte signature to the fleet as the artifact.
//
// validatorNonce is what the kernel will compare against its on-chain
// currentNonce at install time. For a fresh kernel currentNonce is 1, and
// the SDK's getKernelV3Nonce returns 1 for non-existent accounts too; if
// the user installs other plugins between provisioning and the fleet's
// first UserOp this needs to be re-derived.
//
// chainID and kernelAddress are the EIP-712 domain fields the kernel
// reports via eip712Domain() once deployed. For undeployed kernels the
// caller passes the chain id and the deterministic predicted address.
func EnableTypedData(
	chainID *big.Int,
	kernelAddress common.Address,
	validatorIdentifier [21]byte,
	validatorNonce uint32,
	validatorData []byte,
	selectorData []byte,
) signer.TypedData {
	return signer.TypedData{
		Types: signer.Types{
			"EIP712Domain": []signer.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Enable": []signer.Type{
				{Name: "validationId", Type: "bytes21"},
				{Name: "nonce", Type: "uint32"},
				{Name: "hook", Type: "address"},
				{Name: "validatorData", Type: "bytes"},
				{Name: "hookData", Type: "bytes"},
				{Name: "selectorData", Type: "bytes"},
			},
		},
		Domain: signer.TypedDataDomain{
			Name:              KernelName,
			Version:           KernelVersion,
			ChainId:           math.NewHexOrDecimal256(chainID.Int64()),
			VerifyingContract: kernelAddress.String(),
		},
		PrimaryType: "Enable",
		Message: signer.TypedDataMessage{
			"validationId":  validatorIdentifier[:],
			"nonce":         math.NewHexOrDecimal256(int64(validatorNonce)),
			"hook":          common.Address{}.String(),
			"validatorData": validatorData,
			"hookData":      []byte{},
			"selectorData":  selectorData,
		},
	}
}
