package fleet

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// NonceKey builds the 24-byte (uint192) nonce key the kernel uses to route
// a UserOp to a specific validator. Layout, matching the SDK's
// toKernelPluginManager.getNonceKey for EntryPoint v0.7:
//
//	byte 0      : validator mode  (0x00 default / 0x01 enable)
//	byte 1      : validator type  (0x00 sudo / 0x01 secondary)
//	bytes 2..21 : validator identifier (right-padded to 20 bytes — for
//	              weighted-ECDSA, the validator contract address as-is)
//	bytes 22-23 : caller-chosen 16-bit custom key (for sequence isolation)
//
// The full EntryPoint nonce is then (key << 64) | seq, where seq comes from
// EntryPoint.getNonce(account, key).
func NonceKey(mode, validatorType byte, identifier common.Address, customKey uint16) [24]byte {
	var key [24]byte
	key[0] = mode
	key[1] = validatorType
	copy(key[2:22], identifier.Bytes())
	binary.BigEndian.PutUint16(key[22:24], customKey)
	return key
}

// NonceKeyEnableModeForWeightedEcdsa is shorthand for the install-carrying
// UserOp's nonce key.
func NonceKeyEnableModeForWeightedEcdsa(customKey uint16) [24]byte {
	return NonceKey(ValidatorModeEnable, ValidatorTypeSecondary, WeightedEcdsaAddress, customKey)
}

// NonceKeyDefaultModeForWeightedEcdsa is shorthand for every subsequent
// fleet UserOp's nonce key.
func NonceKeyDefaultModeForWeightedEcdsa(customKey uint16) [24]byte {
	return NonceKey(ValidatorModeDefault, ValidatorTypeSecondary, WeightedEcdsaAddress, customKey)
}

// NonceKeyAsUint192 converts the 24-byte key into the *big.Int the
// EntryPoint.getNonce(address,uint192) call accepts.
func NonceKeyAsUint192(key [24]byte) *big.Int {
	return new(big.Int).SetBytes(key[:])
}

// FullNonce composes the EntryPoint's 256-bit nonce from a 24-byte key and
// 8-byte sequence (as returned by EntryPoint.getNonce).
func FullNonce(key [24]byte, seq uint64) *big.Int {
	out := new(big.Int).SetBytes(key[:])
	out.Lsh(out, 64)
	return out.Or(out, new(big.Int).SetUint64(seq))
}
