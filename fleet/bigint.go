package fleet

import "math/big"

// bigU is a tiny helper to turn a uint64 into *big.Int — used as the
// preferred input shape for go-ethereum's abi.Pack of small unsigned types
// like uint24/uint48 where passing a Go uint64 directly works but is more
// implicit.
func bigU(v uint64) *big.Int { return new(big.Int).SetUint64(v) }

// weightsAsBigSlice converts the config's uint32 weights into a slice of
// *big.Int that abi.Pack can consume for a uint24[] argument.
func (c WeightedEcdsaConfig) weightsAsBigSlice() []*big.Int {
	out := make([]*big.Int, len(c.Weights))
	for i, w := range c.Weights {
		out[i] = new(big.Int).SetUint64(uint64(w))
	}
	return out
}
