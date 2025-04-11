package zerodev

import (
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
)

type ClientConfig struct {
	Sender            *common.Address
	SenderSigner      Signer
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
	Sender          *common.Address
	SenderSigner    Signer
	EntryPoint      Entrypoint
	PaymasterClient *PaymasterClient
	BundlerClient   *BundlerClient
	ChainID         *big.Int
	RpcClients      struct {
		Network   *rpc.Client
		Paymaster *rpc.Client
		Bundler   *rpc.Client
	}
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.Sender == nil || config.SenderSigner == nil || config.PaymasterURL == nil || config.BundlerURL == nil || config.EntryPointVersion != EntryPointVersion07 || config.ChainID == nil {
		return nil, errors.New("sender, senderSigner, paymasterURL, bundlerURL, entryPointVersion, and chainID are required")
	}

	networkRpc, err := rpc.Dial(config.RpcURL.String())
	if err != nil {
		return nil, err
	}

	paymasterRpc, err := rpc.Dial(config.PaymasterURL.String())
	if err != nil {
		networkRpc.Close()
		return nil, err
	}

	bundleRpc, err := rpc.Dial(config.BundlerURL.String())
	if err != nil {
		paymasterRpc.Close()
		networkRpc.Close()
		return nil, err
	}

	entrypoint, err := NewEntrypoint07(networkRpc, config.ChainID)
	if err != nil {
		return nil, err
	}

	paymasterClient, err := NewPaymasterClient(paymasterRpc, entrypoint, config.ChainID)
	if err != nil {
		return nil, err
	}

	bundlerClient, err := NewBundlerClient(bundleRpc, entrypoint, config.ChainID)
	if err != nil {
		return nil, err
	}

	return &Client{
		Sender:          config.Sender,
		SenderSigner:    config.SenderSigner,
		PaymasterClient: paymasterClient,
		BundlerClient:   bundlerClient,
		EntryPoint:      entrypoint,
		ChainID:         config.ChainID,
		RpcClients: struct {
			Network   *rpc.Client
			Paymaster *rpc.Client
			Bundler   *rpc.Client
		}{
			Network:   networkRpc,
			Paymaster: paymasterRpc,
			Bundler:   bundleRpc,
		},
	}, nil
}

func (c *Client) Close() {
	c.RpcClients.Network.Close()
	c.RpcClients.Paymaster.Close()
	c.RpcClients.Bundler.Close()
}

// GetUserOperationAndHashToSign creates a UserOperation based on the sender and callData, computes its hash and returns both.
// Allows to create UserOperation with custom sender and then customize the signing process.
// After adding signature to the returned UserOperation, it can be sent by SendSignedUserOperation
func (c *Client) GetUserOperationAndHashToSign(sender *common.Address, callData *[]byte) (*UserOperation, *common.Hash, error) {
	var err error
	var op UserOperation

	nonce, err := c.EntryPoint.GetNonce(*sender)
	if err != nil {
		return nil, nil, err
	}

	op.Sender = sender
	op.Nonce = nonce
	op.CallData = *callData

	gasPrice, err := c.BundlerClient.GetUserOperationGasPrice()
	if err != nil {
		return nil, nil, err
	}

	op.MaxFeePerGas = gasPrice.Standard.MaxFeePerGas
	op.MaxPriorityFeePerGas = gasPrice.Standard.MaxPriorityFeePerGas

	sponsorResponse, err := c.PaymasterClient.SponsorUserOperation(&op)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	return &op, opHash, nil
}

// SendSignedUserOperation sends a pre-signed user operation to the bundler.
// Allows to create UserOperation with different sender and this sender's signature
func (c *Client) SendSignedUserOperation(signedOp *UserOperation) (*UserOperationResult, error) {
	response, err := c.BundlerClient.SendUserOperation(signedOp)
	if err != nil {
		return nil, err
	}

	return &UserOperationResult{
		TxHash: response,
	}, nil
}

// SendUserOperation creates and sends a signed user operation using the provided call data.
// Sender of the user operation is the client's Sender and the signer is SenderSigner
func (c *Client) SendUserOperation(callData *[]byte) (*UserOperationResult, error) {
	op, opHash, err := c.GetUserOperationAndHashToSign(c.Sender, callData)
	if err != nil {
		return nil, err
	}

	signature, err := c.SenderSigner.SignUserOperationHash(*opHash)
	if err != nil {
		return nil, err
	}

	op.Signature = signature

	return c.SendSignedUserOperation(op)
}
