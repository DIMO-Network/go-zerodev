package zerodev

import (
	"context"
	"encoding/json"
	"github.com/DIMO-Network/go-zerodev/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/friendsofgo/errors"
	"math/big"
)

type SponsorUserOperationRequest struct {
	ChainID           *big.Int       `json:"chainId"`
	Operation         *UserOperation `json:"userOp"`
	EntryPointAddress common.Address `json:"entryPointAddress"`
	ShouldOverrideFee bool           `json:"shouldOverrideFee"`
	ShouldConsume     bool           `json:"shouldConsume"`
}

type SponsorUserOperationResponse struct {
	CallGasLimit                  *big.Int `json:"callGasLimit"`
	PaymasterVerificationGasLimit *big.Int `json:"paymasterVerificationGasLimit"`
	PaymasterPostOpGasLimit       *big.Int `json:"paymasterPostOpGasLimit"`
	VerificationGasLimit          *big.Int `json:"verificationGasLimit"`
	MaxPriorityFeePerGas          *big.Int `json:"maxPriorityFeePerGas"`
	Paymaster                     []byte   `json:"paymaster"`
	MaxFeePerGas                  *big.Int `json:"maxFeePerGas"`
	PaymasterData                 []byte   `json:"paymasterData"`
	PreVerificationGas            *big.Int `json:"preVerificationGas"`
}

type SponsorUserOperationResponseHex struct {
	CallGasLimit                  string `json:"callGasLimit"`
	PaymasterVerificationGasLimit string `json:"paymasterVerificationGasLimit"`
	PaymasterPostOpGasLimit       string `json:"paymasterPostOpGasLimit"`
	VerificationGasLimit          string `json:"verificationGasLimit"`
	MaxPriorityFeePerGas          string `json:"maxPriorityFeePerGas"`
	Paymaster                     string `json:"paymaster"`
	MaxFeePerGas                  string `json:"maxFeePerGas"`
	PaymasterData                 string `json:"paymasterData"`
	PreVerificationGas            string `json:"preVerificationGas"`
}

func (r *SponsorUserOperationResponse) MarshalJSON() ([]byte, error) {
	marshal := SponsorUserOperationResponseHex{
		CallGasLimit:                  hexutil.EncodeBig(r.CallGasLimit),
		PaymasterVerificationGasLimit: hexutil.EncodeBig(r.PaymasterVerificationGasLimit),
		PaymasterPostOpGasLimit:       hexutil.EncodeBig(r.PaymasterPostOpGasLimit),
		VerificationGasLimit:          hexutil.EncodeBig(r.VerificationGasLimit),
		MaxPriorityFeePerGas:          hexutil.EncodeBig(r.MaxPriorityFeePerGas),
		Paymaster:                     hexutil.Encode(r.Paymaster),
		MaxFeePerGas:                  hexutil.EncodeBig(r.MaxFeePerGas),
		PaymasterData:                 hexutil.Encode(r.PaymasterData),
		PreVerificationGas:            hexutil.EncodeBig(r.PreVerificationGas),
	}

	return json.Marshal(marshal)
}

func (r *SponsorUserOperationResponse) UnmarshalJSON(b []byte) error {
	var unmarshal SponsorUserOperationResponseHex
	err := json.Unmarshal(b, &unmarshal)
	if err != nil {
		return err
	}

	*r = SponsorUserOperationResponse{
		CallGasLimit:                  big.NewInt(0).SetBytes(common.FromHex(unmarshal.CallGasLimit)),
		PaymasterVerificationGasLimit: big.NewInt(0).SetBytes(common.FromHex(unmarshal.PaymasterVerificationGasLimit)),
		PaymasterPostOpGasLimit:       big.NewInt(0).SetBytes(common.FromHex(unmarshal.PaymasterPostOpGasLimit)),
		VerificationGasLimit:          big.NewInt(0).SetBytes(common.FromHex(unmarshal.VerificationGasLimit)),
		MaxPriorityFeePerGas:          big.NewInt(0).SetBytes(common.FromHex(unmarshal.MaxPriorityFeePerGas)),
		Paymaster:                     common.FromHex(unmarshal.Paymaster),
		MaxFeePerGas:                  big.NewInt(0).SetBytes(common.FromHex(unmarshal.MaxFeePerGas)),
		PaymasterData:                 common.FromHex(unmarshal.PaymasterData),
		PreVerificationGas:            big.NewInt(0).SetBytes(common.FromHex(unmarshal.PreVerificationGas)),
	}

	return nil
}

type PaymasterClient struct {
	Client     types.RPCClient
	EntryPoint Entrypoint
	ChainID    *big.Int
}

func NewPaymasterClient(rpcClient types.RPCClient, entrypoint Entrypoint, chainID *big.Int) (*PaymasterClient, error) {
	if entrypoint == nil || chainID == nil {
		return nil, errors.New("entrypoint, and chainID are required")
	}

	return &PaymasterClient{
		Client:     rpcClient,
		EntryPoint: entrypoint,
		ChainID:    chainID,
	}, nil
}

func (p *PaymasterClient) GetEntryPoint() Entrypoint {
	return p.EntryPoint
}

func (p *PaymasterClient) GetChainID() *big.Int {
	return p.ChainID
}

func (p *PaymasterClient) SponsorUserOperation(op *UserOperation) (*SponsorUserOperationResponse, error) {
	op.Signature = common.FromHex(SignatureDummy)
	return p.sponsor(op)
}

// SponsorUserOperationWithStub is for callers (e.g. the fleet enable-mode
// flow) that need to supply their own stub signature for the paymaster's
// simulation — the default-mode 65-byte dummy won't decode as a valid
// enable-mode signature envelope and the kernel reverts during simulation
// if it doesn't.
//
// The caller is responsible for setting op.Signature to something that
// (1) the validator's getStubSignature path accepts during simulation and
// (2) has the same byte length as the eventual real signature, so that
// gas estimation stays accurate.
func (p *PaymasterClient) SponsorUserOperationWithStub(op *UserOperation) (*SponsorUserOperationResponse, error) {
	if len(op.Signature) == 0 {
		return nil, errors.New("SponsorUserOperationWithStub: caller must set op.Signature to a stub")
	}
	return p.sponsor(op)
}

func (p *PaymasterClient) sponsor(op *UserOperation) (*SponsorUserOperationResponse, error) {
	var request = SponsorUserOperationRequest{
		ChainID:           p.ChainID,
		EntryPointAddress: p.EntryPoint.GetAddress(),
		Operation:         op,
		ShouldOverrideFee: false,
		ShouldConsume:     true,
	}

	var response SponsorUserOperationResponse

	err := p.Client.CallContext(context.Background(), &response, "zd_sponsorUserOperation", request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call zd_sponsorUserOperation")
	}

	return &response, nil
}
