package fleet

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// WeightedEcdsaConfig describes the weighted-ECDSA validator that will be
// installed on a user's kernel as a secondary validator. For DIMO's case
// this is always a single fleet signer with threshold == weight == 100,
// which makes it effectively single-sig — but the on-chain validator is
// still weighted-ECDSA (a different contract than plain ECDSA) so that
// installing it alongside the user's ECDSA sudo doesn't collide on
// (validatorType, validatorContractAddress).
type WeightedEcdsaConfig struct {
	Signers   []common.Address // guardians; must be non-empty
	Weights   []uint32         // same length as Signers; sum >= Threshold
	Threshold uint32           // commonly 100 for single-sig (one signer at weight 100)
	Delay     uint64           // recovery delay; 0 for our flow
}

// EncodeInstallData returns the bytes the weighted-ECDSA validator's
// onInstall() expects: abi.encode(address[] guardians, uint24[] weights,
// uint24 threshold, uint48 delay). This is identical to what the validator
// contract's getEnableData() returns when queried for an already-installed
// kernel, so the same bytes are reusable as the validatorData field of the
// Enable typed-data hash and of the signature envelope.
func (c WeightedEcdsaConfig) EncodeInstallData() ([]byte, error) {
	addrArrT, err := abi.NewType("address[]", "", nil)
	if err != nil {
		return nil, err
	}
	u24ArrT, err := abi.NewType("uint24[]", "", nil)
	if err != nil {
		return nil, err
	}
	u24T, err := abi.NewType("uint24", "", nil)
	if err != nil {
		return nil, err
	}
	u48T, err := abi.NewType("uint48", "", nil)
	if err != nil {
		return nil, err
	}

	args := abi.Arguments{
		{Name: "_guardians", Type: addrArrT},
		{Name: "_weights", Type: u24ArrT},
		{Name: "_threshold", Type: u24T},
		{Name: "_delay", Type: u48T},
	}

	// abi.Pack with uint24/uint48 wants *big.Int for the scalar values and
	// concrete address slices / uint32 slices for the arrays — but the
	// ABI encoder accepts wider Go ints and truncates as long as they fit.
	// We feed *big.Int values to be unambiguous.
	return args.Pack(c.Signers, c.weightsAsBigSlice(), bigU(uint64(c.Threshold)), bigU(c.Delay))
}

// ValidationId returns the 21-byte vId that identifies this validator
// inside the kernel's storage: 0x01 (SECONDARY) || weighted-ECDSA address.
func ValidationIdForWeightedEcdsa() [21]byte {
	var v [21]byte
	v[0] = ValidatorTypeSecondary
	copy(v[1:], WeightedEcdsaAddress.Bytes())
	return v
}
