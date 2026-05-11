package fleet

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// EnableEnvelope is the signature wrapper attached to an enable-mode UserOp.
// The kernel decodes this off the front of the UserOp.signature field to
// learn (a) what plugin to install, (b) which sudo approved the install,
// and (c) the inner signature the regular validator should verify.
type EnableEnvelope struct {
	// Hook contract that gates calls into the regular validator's slot.
	// Zero in our flow.
	Hook common.Address
	// Bytes the validator's onInstall() receives — same as
	// WeightedEcdsaConfig.EncodeInstallData.
	ValidatorData []byte
	// Bytes the hook's onInstall() receives — empty in our flow.
	HookData []byte
	// The selectorData blob (action selector + target + hook + abi-encoded
	// init data).
	SelectorData []byte
	// 65-byte ECDSA signature the user's sudo produced over the Enable
	// typed-data hash (the artifact's enableSignature field).
	EnableSig []byte
	// 65-byte ECDSA signature the fleet's weighted-ECDSA signer produced
	// over the EIP-191 wrapping of the userOpHash.
	UserOpSig []byte
}

// Encode produces the bytes that should go into UserOperation.Signature for
// the enable-mode UserOp. Layout, mirroring the SDK's getEncodedPluginsData
// for EntryPoint v0.7:
//
//	hook (20 bytes)
//	abi.encode(bytes validatorData, bytes hookData, bytes selectorData,
//	           bytes enableSig, bytes userOpSig)
func (e EnableEnvelope) Encode() ([]byte, error) {
	bytesT, err := abi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	tail, err := (abi.Arguments{
		{Name: "validatorData", Type: bytesT},
		{Name: "hookData", Type: bytesT},
		{Name: "selectorData", Type: bytesT},
		{Name: "enableSig", Type: bytesT},
		{Name: "userOpSig", Type: bytesT},
	}).Pack(e.ValidatorData, e.HookData, e.SelectorData, e.EnableSig, e.UserOpSig)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(e.Hook.Bytes())
	buf.Write(tail)
	return buf.Bytes(), nil
}
