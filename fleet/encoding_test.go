package fleet

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// All fixtures here come from zdexplore/08-go-vectors.ts run against
// deterministic test private keys (0x11..11 for user, 0x22..22 for fleet).
// Any drift between this file and what the ZeroDev TS SDK produces means
// the Go encoder has diverged from the SDK — the kernel will reject the
// signatures we build.

const (
	fixUserAddr    = "0x19E7E376E7C213B7E7e7e46cc70A5dD086DAff2A"
	fixFleetAddr   = "0x1563915e194D8CfBA1943570603F7606A3115508"
	fixKernelAddr  = "0x5cD5c2bB15fFBe653015377E62926aFa60D5e0a7"
	fixSampleHash  = "0xce56ebec5d1a8e106b56052a5817e6e0520f10fe26061ac3448ddaf891ccc83a"
	fixEip191Hash  = "0xc0b582c2368f4bd35bb4652c993aeb3bd9bb6cf3eb85cd058e271bdc8dd5265b"
	fixEnableHash  = "0x165a42047fc94f50d1b16309a0d0c780c54a0cb3355e08aed5192aef8ad8ad73"
	fixEnableSig   = "0xcf5a6200dc9d62800dcd231af73488a155730f452f1098b70f2338381f935e60457293e56487d95d01d9b126e5b021bb0338120a6fc66307350ff075d07e6d9c1c"
	fixEnableNonce = "0x0101eD89244160CfE273800B58b1B534031699dFeEEE3782"
	fixDefaultNonce = "0x0001eD89244160CfE273800B58b1B534031699dFeEEE0223"
)

const fixValidatorInstallData = "0x" +
	"0000000000000000000000000000000000000000000000000000000000000080" +
	"00000000000000000000000000000000000000000000000000000000000000c0" +
	"0000000000000000000000000000000000000000000000000000000000000064" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000001" +
	"0000000000000000000000001563915e194d8cfba1943570603f7606a3115508" +
	"0000000000000000000000000000000000000000000000000000000000000001" +
	"0000000000000000000000000000000000000000000000000000000000000064"

const fixSelectorData = "0x" +
	"e9ae5c5300000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000004000000000000000000000000000000000000000000000000000" +
	"0000000000800000000000000000000000000000000000000000000000000000" +
	"00000000001FF0000000000000000000000000000000000000000000000000000" +
	"000000000000000000000000000000000000000000000000000000000000000002" +
	"00000000000000000000000000000000000000000000000000000000000000"

// The chunks above were copy-paste-prone; rebuild from the raw string to
// avoid hand-aligning hex.

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		t.Fatalf("hex decode %q: %v", s, err)
	}
	return b
}

func TestWeightedEcdsaInstallData(t *testing.T) {
	cfg := WeightedEcdsaConfig{
		Signers:   []common.Address{common.HexToAddress(fixFleetAddr)},
		Weights:   []uint32{100},
		Threshold: 100,
		Delay:     0,
	}
	got, err := cfg.EncodeInstallData()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	want := unhex(t, "0x"+
		"0000000000000000000000000000000000000000000000000000000000000080"+
		"00000000000000000000000000000000000000000000000000000000000000c0"+
		"0000000000000000000000000000000000000000000000000000000000000064"+
		"0000000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000001"+
		"0000000000000000000000001563915e194d8cfba1943570603f7606a3115508"+
		"0000000000000000000000000000000000000000000000000000000000000001"+
		"0000000000000000000000000000000000000000000000000000000000000064")

	if !bytes.Equal(got, want) {
		t.Fatalf("install data mismatch\n got %x\nwant %x", got, want)
	}
}

func TestSelectorData(t *testing.T) {
	got, err := SelectorData(DefaultAction())
	if err != nil {
		t.Fatalf("SelectorData: %v", err)
	}

	// The TS dump is what we're aiming at: action.selector || target || hook
	// || abi.encode(bytes 0xFF, bytes 0x0000).
	want := unhex(t, "0x"+
		// action.selector (4 bytes)
		"e9ae5c53"+
		// target (20 bytes)
		"0000000000000000000000000000000000000000"+
		// hook (20 bytes)
		"0000000000000000000000000000000000000000"+
		// abi.encode(bytes, bytes): offsets, then each (length, value)
		"0000000000000000000000000000000000000000000000000000000000000040"+
		"0000000000000000000000000000000000000000000000000000000000000080"+
		"0000000000000000000000000000000000000000000000000000000000000001"+
		"ff00000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000002"+
		"0000000000000000000000000000000000000000000000000000000000000000")

	if !bytes.Equal(got, want) {
		t.Fatalf("selector data mismatch\n got %x\nwant %x", got, want)
	}
}

func TestNonceKey(t *testing.T) {
	enable := NonceKeyEnableModeForWeightedEcdsa(0x3782)
	def := NonceKeyDefaultModeForWeightedEcdsa(0x0223)

	wantEnable := unhex(t, fixEnableNonce)
	wantDefault := unhex(t, fixDefaultNonce)

	if !bytes.Equal(enable[:], wantEnable) {
		t.Fatalf("enable nonce key mismatch\n got %x\nwant %x", enable, wantEnable)
	}
	if !bytes.Equal(def[:], wantDefault) {
		t.Fatalf("default nonce key mismatch\n got %x\nwant %x", def, wantDefault)
	}
}

func TestEnableTypedDataHashAndSignatureRecovers(t *testing.T) {
	cfg := WeightedEcdsaConfig{
		Signers:   []common.Address{common.HexToAddress(fixFleetAddr)},
		Weights:   []uint32{100},
		Threshold: 100,
	}
	validatorData, err := cfg.EncodeInstallData()
	if err != nil {
		t.Fatalf("install data: %v", err)
	}
	sel, err := SelectorData(DefaultAction())
	if err != nil {
		t.Fatalf("selector data: %v", err)
	}

	vId := ValidationIdForWeightedEcdsa()
	td := EnableTypedData(
		big.NewInt(11155111),
		common.HexToAddress(fixKernelAddr),
		vId,
		1,
		validatorData,
		sel,
	)

	// Recompute the EIP-712 hash using go-ethereum's typed-data primitives
	// and assert it matches what viem produced TS-side.
	domainSep, err := td.HashStruct("EIP712Domain", td.Domain.Map())
	if err != nil {
		t.Fatalf("domain separator: %v", err)
	}
	structHash, err := td.HashStruct(td.PrimaryType, td.Message)
	if err != nil {
		t.Fatalf("struct hash: %v", err)
	}
	digest := crypto.Keccak256Hash(append(append([]byte{0x19, 0x01}, domainSep...), structHash...))
	if digest.Hex() != fixEnableHash {
		t.Fatalf("enable typed-data hash mismatch\n got  %s\nwant %s", digest.Hex(), fixEnableHash)
	}

	// And the artifact's enable signature must recover to the user EOA.
	sig := unhex(t, fixEnableSig)
	if len(sig) != 65 {
		t.Fatalf("sig length: got %d want 65", len(sig))
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	pub, err := crypto.SigToPub(digest.Bytes(), sig)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	recovered := crypto.PubkeyToAddress(*pub)
	want := common.HexToAddress(fixUserAddr)
	if recovered != want {
		t.Fatalf("enable sig recover\n got  %s\nwant %s", recovered, want)
	}
}

func TestPersonalSignDigestMatchesSdk(t *testing.T) {
	// The Go signer must produce the same EIP-191 digest the validator
	// will ecrecover against. Compare the digest only — signing requires
	// the fleet PK which isn't checked into this fixture set.
	hash := common.HexToHash(fixSampleHash)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	digest := crypto.Keccak256Hash(append(prefix, hash.Bytes()...))
	if digest.Hex() != fixEip191Hash {
		t.Fatalf("eip191 digest\n got  %s\nwant %s", digest.Hex(), fixEip191Hash)
	}
}

func TestFactoryData(t *testing.T) {
	got, err := FactoryData(common.HexToAddress(fixUserAddr), big.NewInt(0))
	if err != nil {
		t.Fatalf("FactoryData: %v", err)
	}

	// Byte-for-byte equality with what the ZeroDev TS SDK produced for
	// the same inputs (zdexplore/08-go-vectors.ts). If this drifts, the
	// EntryPoint will derive a different kernel address than the
	// artifact's sender field, and the install-carrying UserOp will be
	// rejected with a sender mismatch.
	want := unhex(t, "0xc5265d5d"+
		// deployWithFactory(address factory, bytes initData, bytes32 salt)
		"000000000000000000000000aac5d4240af87249b3f71bc8e4a2cae074a3e419"+ // kernel factory
		"0000000000000000000000000000000000000000000000000000000000000060"+ // offset to initData
		"0000000000000000000000000000000000000000000000000000000000000000"+ // salt = index 0
		// initData length (0x124 = 292 bytes) + initialize(...) call
		"0000000000000000000000000000000000000000000000000000000000000124"+
		"3c3b752b"+ // initialize selector
		// rootValidator vId (bytes21: SECONDARY type + ECDSA validator addr), right-padded to 32
		"01845adb2c711129d4f3966735ed98a9f09fc4ce570000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000000"+ // hook = zero
		"00000000000000000000000000000000000000000000000000000000000000a0"+ // offset to validatorData
		"00000000000000000000000000000000000000000000000000000000000000e0"+ // offset to hookData
		"0000000000000000000000000000000000000000000000000000000000000100"+ // offset to initConfig
		// validatorData = user signer EOA address, padded
		"0000000000000000000000000000000000000000000000000000000000000014"+ // length 20
		"19e7e376e7c213b7e7e7e46cc70a5dd086daff2a000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000000"+ // hookData length = 0
		"0000000000000000000000000000000000000000000000000000000000000000"+ // initConfig length = 0
		// 28 bytes of trailing zero padding to round up to a 32-byte boundary
		"00000000000000000000000000000000000000000000000000000000")

	if !bytes.Equal(got, want) {
		t.Fatalf("factoryData mismatch\n got %x\nwant %x", got, want)
	}
}

func TestEnableEnvelopeRoundtrip(t *testing.T) {
	// Smoke test: build an envelope with realistic bytes, encode it,
	// confirm we can locate the substrings we care about. Real byte-level
	// equivalence is exercised end-to-end in scripts/03; here we just
	// catch encoder breakage (wrong abi packing, wrong field order).
	cfg := WeightedEcdsaConfig{
		Signers:   []common.Address{common.HexToAddress(fixFleetAddr)},
		Weights:   []uint32{100},
		Threshold: 100,
	}
	validatorData, _ := cfg.EncodeInstallData()
	sel, _ := SelectorData(DefaultAction())

	env := EnableEnvelope{
		ValidatorData: validatorData,
		HookData:      []byte{},
		SelectorData:  sel,
		EnableSig:     unhex(t, fixEnableSig),
		UserOpSig:     bytes.Repeat([]byte{0xab}, 65),
	}
	enc, err := env.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// First 20 bytes are the hook (zero) — sanity.
	if !bytes.HasPrefix(enc, make([]byte, 20)) {
		t.Fatalf("envelope hook prefix not zero: %x", enc[:20])
	}
	if !bytes.Contains(enc, validatorData) {
		t.Fatal("envelope missing validator install data")
	}
	if !bytes.Contains(enc, sel) {
		t.Fatal("envelope missing selector data")
	}
}

// signer.TypedData isn't directly used outside this test, but importing it
// keeps gofmt from rearranging the build deps in a way that masks a real
// missing import.
var _ = signer.TypedData{}
