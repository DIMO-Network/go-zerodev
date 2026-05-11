package fleet

import (
	"context"
	"strings"

	"github.com/DIMO-Network/go-zerodev/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const isAllowedSelectorABI = `[{
    "type":"function","name":"isAllowedSelector","stateMutability":"view",
    "inputs":[
        {"name":"vId","type":"bytes21"},
        {"name":"selector","type":"bytes4"}
    ],"outputs":[{"name":"","type":"bool"}]
}]`

// validationIdForWeightedEcdsa returns the 21-byte vId that identifies the
// weighted-ECDSA validator inside the kernel's storage: 0x01 (SECONDARY)
// followed by the validator contract address.
func validationIdForWeightedEcdsa() [21]byte {
	var v [21]byte
	v[0] = ValidatorTypeSecondary
	copy(v[1:], WeightedEcdsaAddress.Bytes())
	return v
}

// IsFleetInstalled returns true when (a) the kernel is deployed AND (b)
// the weighted-ECDSA validator is installed with selector permission for
// executeUserOp. Useful as a sanity check before sending the first UserOp
// against a kernel the admin claimed to provision — if either side
// returns false, something on the admin side didn't complete and a
// SendCall will fail.
//
// The deployment check is folded in because isAllowedSelector reverts on
// a non-existent contract.
func IsFleetInstalled(ctx context.Context, rpc types.RPCClient, kernel common.Address) (bool, error) {
	code, err := getCode(ctx, rpc, kernel)
	if err != nil {
		return false, err
	}
	if len(code) == 0 {
		return false, nil
	}

	parsed, err := abi.JSON(strings.NewReader(isAllowedSelectorABI))
	if err != nil {
		return false, err
	}
	vId := validationIdForWeightedEcdsa()
	callData, err := parsed.Pack("isAllowedSelector", vId, ExecuteUserOpSelector)
	if err != nil {
		return false, err
	}

	var result hexutil.Bytes
	msg := struct {
		To   common.Address `json:"to"`
		Data hexutil.Bytes  `json:"data"`
	}{To: kernel, Data: callData}
	if err := rpc.CallContext(ctx, &result, "eth_call", msg, "latest"); err != nil {
		return false, err
	}

	out, err := parsed.Unpack("isAllowedSelector", result)
	if err != nil {
		return false, err
	}
	allowed, _ := out[0].(bool)
	return allowed, nil
}

func getCode(ctx context.Context, rpc types.RPCClient, addr common.Address) ([]byte, error) {
	var code hexutil.Bytes
	if err := rpc.CallContext(ctx, &code, "eth_getCode", addr, "latest"); err != nil {
		return nil, err
	}
	return code, nil
}
