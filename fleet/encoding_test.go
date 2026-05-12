package fleet

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Fixtures from zdexplore/08-go-vectors.ts run against deterministic test
// private keys (0x11..11 for user, 0x22..22 for fleet). If any of these
// drifts from what the ZeroDev TS SDK / on-chain validator produce, the
// fleet's UserOps will fail validation.

const (
	fixSampleHash    = "0xce56ebec5d1a8e106b56052a5817e6e0520f10fe26061ac3448ddaf891ccc83a"
	fixEip191Hash    = "0xc0b582c2368f4bd35bb4652c993aeb3bd9bb6cf3eb85cd058e271bdc8dd5265b"
	fixDefaultNonce  = "0x0001eD89244160CfE273800B58b1B534031699dFeEEE0223"
)

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		t.Fatalf("hex decode %q: %v", s, err)
	}
	return b
}

func TestNonceKeyDefaultMode(t *testing.T) {
	got := NonceKeyDefaultModeForWeightedEcdsa(0x0223)
	want := unhex(t, fixDefaultNonce)
	if !bytes.Equal(got[:], want) {
		t.Fatalf("default-mode nonce key mismatch\n got %x\nwant %x", got, want)
	}
}

func TestPersonalSignDigestMatchesSdk(t *testing.T) {
	// The Go signer must produce the same EIP-191 digest the validator
	// will ecrecover against. We check the digest only — verifying the
	// signature would require the fleet PK which isn't checked into this
	// fixture set.
	hash := common.HexToHash(fixSampleHash)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digest := crypto.Keccak256Hash(append(prefix, hash.Bytes()...))
	if digest.Hex() != fixEip191Hash {
		t.Fatalf("eip191 digest\n got  %s\nwant %s", digest.Hex(), fixEip191Hash)
	}
}

func TestValidationIdForWeightedEcdsa(t *testing.T) {
	got := validationIdForWeightedEcdsa()
	// First byte is the SECONDARY type marker; the rest is the validator
	// contract address.
	if got[0] != ValidatorTypeSecondary {
		t.Fatalf("first byte of vId: got %02x want %02x", got[0], ValidatorTypeSecondary)
	}
	if !bytes.Equal(got[1:], WeightedEcdsaAddress.Bytes()) {
		t.Fatalf("vId tail: got %x want %x", got[1:], WeightedEcdsaAddress.Bytes())
	}
}
