package fleet

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// PersonalSignUserOpHash returns the 65-byte signature the weighted-ECDSA
// validator expects from a fleet signer when validating a UserOp.
//
// The validator does NOT verify a raw secp256k1 signature over userOpHash.
// It calls signMessage({ raw: userOpHash }) on the signer in the TS SDK,
// which is EIP-191 personal_sign: keccak256("\x19Ethereum Signed Message:\n32"
// || userOpHash), then ECDSA-sign that. The on-chain validator does the
// equivalent ecrecover (it strips the EIP-191 prefix before recovering),
// so the signature must be produced over the prefixed digest.
//
// This is the most likely place to get the signing wrong — the existing
// account.SmartAccountPrivateKeySigner in this module signs the userOpHash
// directly, which is correct for the sudo ECDSA validator but wrong for
// the weighted-ECDSA secondary validator we use for fleet keys. Hence a
// separate helper.
func PersonalSignUserOpHash(pk *ecdsa.PrivateKey, userOpHash common.Hash) ([]byte, error) {
	if pk == nil {
		return nil, fmt.Errorf("fleet: nil private key")
	}
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(userOpHash))
	digest := crypto.Keccak256Hash(append([]byte(prefix), userOpHash.Bytes()...))

	sig, err := crypto.Sign(digest.Bytes(), pk)
	if err != nil {
		return nil, err
	}
	// go-ethereum returns v in {0,1}; Ethereum signature convention is
	// {27,28}. The kernel + weighted-ECDSA validator both expect the
	// {27,28} form (they internally subtract 27 before ecrecover).
	sig[64] += 27
	return sig, nil
}
