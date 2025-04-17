package zerodev

import (
	"context"
	"encoding/json"
	"github.com/DIMO-Network/go-zerodev/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/friendsofgo/errors"
	"math/big"
	"time"
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

type UserOperationReceipt struct {
	TransactionHash   *hexutil.Bytes  `json:"transactionHash"`
	TransactionIndex  *hexutil.Big    `json:"transactionIndex"`
	BlockHash         *hexutil.Bytes  `json:"blockHash"`
	BlockNumber       *hexutil.Big    `json:"blockNumber"`
	From              common.Address  `json:"from"`
	To                common.Address  `json:"to"`
	CumulativeGasUsed *hexutil.Big    `json:"cumulativeGasUsed"`
	GasUsed           *hexutil.Big    `json:"gasUsed"`
	ContractAddress   *common.Address `json:"contractAddress"`
	Logs              []ethtypes.Log  `json:"logs"`
	LogsBloom         *hexutil.Bytes  `json:"logsBloom"`
	Status            *hexutil.Uint   `json:"status"`
	EffectiveGasPrice *hexutil.Big    `json:"effectiveGasPrice"`
}
type GetUserOperationReceiptResponse struct {
	UserOpHash    *hexutil.Bytes       `json:"userOpHash"`
	Entrypoint    common.Address       `json:"entrypoint"`
	Sender        common.Address       `json:"sender"`
	Nonce         *hexutil.Bytes       `json:"nonce"`
	Paymaster     common.Address       `json:"paymaster"`
	ActualGasUsed *hexutil.Big         `json:"actualGasUsed"`
	ActualGasCost *hexutil.Big         `json:"actualGasCost"`
	Success       bool                 `json:"success"`
	Logs          []ethtypes.Log       `json:"logs"`
	Receipt       UserOperationReceipt `json:"receipt"`
}

type BundlerClient struct {
	Client     types.RPCClient
	EntryPoint Entrypoint
	ChainID    *big.Int
}

func NewBundlerClient(rpcClient types.RPCClient, entrypoint Entrypoint, chainID *big.Int) (*BundlerClient, error) {
	if entrypoint == nil || chainID == nil {
		return nil, errors.New("entrypoint, and chainID are required")
	}

	return &BundlerClient{
		Client:     rpcClient,
		EntryPoint: entrypoint,
		ChainID:    chainID,
	}, nil
}

func (b *BundlerClient) GetEntryPoint() Entrypoint {
	return b.EntryPoint
}

func (b *BundlerClient) GetChainID() *big.Int {
	return b.ChainID
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

func (b *BundlerClient) SendUserOperation(op *UserOperation) ([]byte, error) {
	var hex hexutil.Bytes

	err := b.Client.CallContext(context.Background(), &hex, "eth_sendUserOperation", op, b.EntryPoint.GetAddress())
	if err != nil {
		return nil, err
	}

	var response []byte = hex
	return response, nil
}

func (b *BundlerClient) GetUserOperationReceipt(hash []byte) (*UserOperationReceipt, error) {
	var response GetUserOperationReceiptResponse
	ctx := context.Background()

	for i := 0; i < 24; i++ {
		err := b.Client.CallContext(ctx, &response, "eth_getUserOperationReceipt", hexutil.Encode(hash))
		if err != nil {
			return nil, err
		}
		if response.UserOpHash == nil {
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	if response.UserOpHash == nil {
		return nil, errors.New("failed to get receipt for user operation: " + hexutil.Encode(hash))
	}

	return &response.Receipt, nil
}
