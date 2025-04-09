package zerodev

import (
	"errors"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

type ClientConfig struct {
	Signer            Signer
	EntryPointVersion string
	RpcURL            *url.URL
	PaymasterURL      *url.URL
	BundlerURL        *url.URL
	ChainID           *big.Int
}

type UserOperationResult struct {
	TxHash *[]byte `json:"txHash"`
}

type Client struct {
	Signer          Signer
	EntryPoint      Entrypoint
	PaymasterClient *PaymasterClient
	BundlerClient   *BundlerClient
	ChainID         *big.Int
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.Signer == nil || config.PaymasterURL == nil || config.BundlerURL == nil || config.EntryPointVersion != EntryPointVersion07 || config.ChainID == nil {
		return nil, errors.New("signer, paymasterURL, bundlerURL, entryPoint, and chainID are required")
	}

	entrypoint, err := NewEntrypoint07(config.RpcURL, config.ChainID)
	if err != nil {
		return nil, err
	}

	paymasterClient, err := NewPaymasterClient(config.PaymasterURL, entrypoint, config.ChainID)
	if err != nil {
		return nil, err
	}

	bundlerClient, err := NewBundlerClient(config.BundlerURL, entrypoint, config.ChainID)
	if err != nil {
		return nil, err
	}

	return &Client{
		Signer:          config.Signer,
		PaymasterClient: paymasterClient,
		BundlerClient:   bundlerClient,
		EntryPoint:      entrypoint,
		ChainID:         config.ChainID,
	}, nil
}

func (c *Client) Close() {
	c.PaymasterClient.Close()
	c.BundlerClient.Close()
	c.EntryPoint.Close()
}

func (c *Client) SendUserOperation(sender common.Address, callData *[]byte) (*UserOperationResult, error) {
	var err error
	var op UserOperation

	nonce, err := c.EntryPoint.GetNonce(&sender)
	if err != nil {
		return nil, err
	}

	op.Sender = &sender
	op.Nonce = nonce
	op.CallData = *callData

	gasPrice, err := c.BundlerClient.GetUserOperationGasPrice()
	if err != nil {
		return nil, err
	}

	op.MaxFeePerGas = gasPrice.Standard.MaxFeePerGas
	op.MaxPriorityFeePerGas = gasPrice.Standard.MaxPriorityFeePerGas

	sponsorResponse, err := c.PaymasterClient.SponsorUserOperation(&op)
	if err != nil {
		return nil, err
	}

	op.Paymaster = sponsorResponse.Paymaster
	op.PaymasterData = sponsorResponse.PaymasterData
	op.PreVerificationGas = sponsorResponse.PreVerificationGas
	op.VerificationGasLimit = sponsorResponse.VerificationGasLimit
	op.PaymasterVerificationGasLimit = sponsorResponse.PaymasterVerificationGasLimit
	op.PaymasterPostOpGasLimit = sponsorResponse.PaymasterPostOpGasLimit
	op.CallGasLimit = sponsorResponse.CallGasLimit

	opHash, err := c.EntryPoint.GetUserOperationHash(&op)
	if err != nil {
		return nil, err
	}

	signature, err := c.Signer.SignHash(*opHash)
	if err != nil {
		return nil, err
	}

	op.Signature = signature

	response, err := c.BundlerClient.SendUserOperation(&op)
	if err != nil {
		return nil, err
	}

	return &UserOperationResult{
		TxHash: response,
	}, nil
}
