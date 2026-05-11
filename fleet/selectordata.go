package fleet

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Action describes the kernel selector + target a secondary validator is
// being granted permission to validate UserOps for. For our flow the
// validator is granted permission for the default executeUserOp selector
// with no target hook — i.e. exactly what the SDK produces by default.
type Action struct {
	Selector [4]byte        // first 4 bytes of the kernel function being authorized
	Target   common.Address // target contract for the action (zero for executeUserOp)
	Hook     common.Address // hook contract for the action (zero for "no hook")
}

// DefaultAction returns the standard {executeUserOp, zero, zero} action
// the SDK uses when nothing custom is configured. This is what we want for
// a vanilla "fleet can execute arbitrary calls" install.
func DefaultAction() Action {
	return Action{Selector: ExecuteUserOpSelector}
}

// SelectorData encodes the action blob that appears both inside the Enable
// typed-data hash (as the selectorData field) AND inside the signature
// envelope. Layout, mirroring the SDK's getPluginsEnableTypedData /
// getEncodedPluginsData:
//
//	action.selector              (4  bytes)
//	action.target                (20 bytes)
//	action.hook                  (20 bytes)
//	abi.encode(bytes selectorInitData, bytes hookInitData)
//	    where selectorInitData = 0xFF (CALL_TYPE.DELEGATE_CALL),
//	          hookInitData     = 0x0000.
//
// The trailing abi.encode'd suffix is a fixed 192-byte tail in our case
// because both bytes args are short and fit in one word each.
func SelectorData(a Action) ([]byte, error) {
	bytesT, err := abi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	tail, err := (abi.Arguments{
		{Name: "selectorInitData", Type: bytesT},
		{Name: "hookInitData", Type: bytesT},
	}).Pack(CallTypeDelegateCall, []byte{0x00, 0x00})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(a.Selector[:])
	buf.Write(a.Target.Bytes())
	buf.Write(a.Hook.Bytes())
	buf.Write(tail)
	return buf.Bytes(), nil
}
