package zerodev

import (
	"context"
	"encoding/json"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/friendsofgo/errors"
)

type GasPriceSpecification struct {
	MaxPriorityFeePerGas *big.Int `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *big.Int `json:"maxFeePerGas"`
}

type GasPriceSpecificationHex struct {
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
}

func (g *GasPriceSpecification) UnmarshalJSON(b []byte) error {
	var unmarshal GasPriceSpecificationHex
	err := json.Unmarshal(b, &unmarshal)
	if err != nil {
		return err
	}

	*g = GasPriceSpecification{
		MaxPriorityFeePerGas: big.NewInt(0).SetBytes(common.FromHex(unmarshal.MaxPriorityFeePerGas)),
		MaxFeePerGas:         big.NewInt(0).SetBytes(common.FromHex(unmarshal.MaxFeePerGas)),
	}

	return nil
}

type GetUserOperationGasPriceResponse struct {
	Slow     *GasPriceSpecification `json:"slow"`
	Standard *GasPriceSpecification `json:"standard"`
	Fast     *GasPriceSpecification `json:"fast"`
}

type SendUserOperationRequest struct {
	ChainID           *uint64         `json:"chainId"`
	Operation         *UserOperation  `json:"userOp"`
	EntryPointAddress *common.Address `json:"entryPointAddress"`
	GasToken          *common.Address `json:"gasToken,omitempty"`
	ShouldOverrideFee bool            `json:"shouldOverrideFee"`
	ShouldConsume     bool            `json:"shouldConsume"`
}

type SendUserOperationResponse struct {
	TxHash *hexutil.Bytes `json:"txHash"`
}

type BundlerClient struct {
	URL        *url.URL
	Client     *rpc.Client
	EntryPoint Entrypoint
	ChainID    *big.Int
}

func NewBundlerClient(url *url.URL, entrypoint Entrypoint, chainID *big.Int) (*BundlerClient, error) {
	if url == nil || entrypoint == nil || chainID == nil {
		return nil, errors.New("url, entrypoint, and chainID are required")
	}

	client, err := rpc.Dial(url.String())
	if err != nil {
		return nil, err
	}

	return &BundlerClient{
		URL:        url,
		Client:     client,
		EntryPoint: entrypoint,
		ChainID:    chainID,
	}, nil
}

func (b *BundlerClient) Close() {
	b.Client.Close()
}

func (b *BundlerClient) GetEntryPoint() Entrypoint {
	return b.EntryPoint
}

func (b *BundlerClient) GetChainID() *big.Int {
	return b.ChainID
}

func (b *BundlerClient) GetURL() *url.URL {
	return b.URL
}

func (b *BundlerClient) GetClient() *rpc.Client {
	return b.Client
}

func (b *BundlerClient) GetUserOperationGasPrice() (*GetUserOperationGasPriceResponse, error) {
	var err error
	var response GetUserOperationGasPriceResponse

	err = b.Client.CallContext(context.Background(), &response, "zd_getUserOperationGasPrice")
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (b *BundlerClient) SendUserOperation(op *UserOperation) (*[]byte, error) {
	var hex hexutil.Bytes

	err := b.Client.CallContext(context.Background(), &hex, "eth_sendUserOperation", op, b.EntryPoint.GetAddress())
	if err != nil {
		return nil, err
	}

	var response []byte = hex
	return &response, nil
}
