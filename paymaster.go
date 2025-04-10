package zerodev

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/friendsofgo/errors"
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
	Client     *rpc.Client
	EntryPoint Entrypoint
	ChainID    *big.Int
}

func NewPaymasterClient(rpcClient *rpc.Client, entrypoint Entrypoint, chainID *big.Int) (*PaymasterClient, error) {
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
		return nil, err
	}

	return &response, nil
}
