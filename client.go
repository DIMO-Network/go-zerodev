package zerodev

import (
	"crypto/ecdsa"
	"errors"
	"github.com/DIMO-Network/go-zerodev/account"
	"github.com/DIMO-Network/go-zerodev/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"net/url"
)

type ClientConfig struct {
	AccountAddress             common.Address
	AccountPK                  *ecdsa.PrivateKey
	EntryPointVersion          string
	RpcURL                     *url.URL
	PaymasterURL               *url.URL
	BundlerURL                 *url.URL
	ChainID                    *big.Int
	ReceiptPollingDelaySeconds int
	ReceiptPollingRetries      int
}

type UserOperationResult struct {
	UserOperationHash []byte                `json:"userOperationHash"`
	Receipt           *UserOperationReceipt `json:"receipt,omitempty"`
}

type Client struct {
	Signer          types.AccountSigner
	EntryPoint      Entrypoint
	PaymasterClient *PaymasterClient
	BundlerClient   *BundlerClient
	ChainID         *big.Int
	RpcClients      struct {
		Network   *rpc.Client
		Paymaster *rpc.Client
		Bundler   *rpc.Client
	}
	ReceiptPollingDelay   int
	ReceiptPollingRetries int
}

func NewClient(config *ClientConfig) (*Client, error) {
	if config.AccountPK == nil || config.PaymasterURL == nil || config.BundlerURL == nil || config.EntryPointVersion != EntryPointVersion07 || config.ChainID == nil {
		return nil, errors.New("accountPK, paymasterURL, bundlerURL, entryPointVersion and chainID are required")
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
		networkRpc.Close()
		paymasterRpc.Close()
		networkRpc.Close()
		return nil, err
	}

	paymasterClient, err := NewPaymasterClient(paymasterRpc, entrypoint, config.ChainID)
	if err != nil {
		networkRpc.Close()
		paymasterRpc.Close()
		networkRpc.Close()
		return nil, err
	}

	bundlerClient, err := NewBundlerClient(bundleRpc, entrypoint, config.ChainID)
	if err != nil {
		networkRpc.Close()
		paymasterRpc.Close()
		networkRpc.Close()
		return nil, err
	}

	signer, err := account.NewSmartAccountPrivateKeySigner(networkRpc, config.AccountAddress, config.AccountPK)
	if err != nil {
		networkRpc.Close()
		paymasterRpc.Close()
		networkRpc.Close()
		return nil, err
	}

	pollingDelaySeconds := 10
	if config.ReceiptPollingDelaySeconds > 0 {
		pollingDelaySeconds = config.ReceiptPollingDelaySeconds
	}

	pollingRetries := 24
	if config.ReceiptPollingRetries > 0 {
		pollingRetries = config.ReceiptPollingRetries
	}

	return &Client{
		Signer:          signer,
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
		ReceiptPollingDelay:   pollingDelaySeconds,
		ReceiptPollingRetries: pollingRetries,
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
func (c *Client) GetUserOperationAndHashToSign(sender common.Address, callData *[]byte) (*UserOperation, *common.Hash, error) {
	var err error
	var op UserOperation

	nonce, err := c.EntryPoint.GetNonce(sender)
	if err != nil {
		return nil, nil, err
	}

	op.Sender = &sender
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
func (c *Client) SendSignedUserOperation(signedOp *UserOperation, waitForReceipt bool) (*UserOperationResult, error) {
	response, err := c.BundlerClient.SendUserOperation(signedOp)
	if err != nil {
		return nil, err
	}

	var receipt *UserOperationReceipt

	if waitForReceipt {
		receipt, _ = c.BundlerClient.GetUserOperationReceipt(response, c.ReceiptPollingDelay, c.ReceiptPollingRetries)
	}

	return &UserOperationResult{
		UserOperationHash: response,
		Receipt:           receipt,
	}, nil
}

// SendUserOperation creates and sends a signed user operation using the provided call data.
// Sender of the user operation is the client's Sender and the signer is SenderSigner
func (c *Client) SendUserOperation(callData *[]byte, waitForReceipt bool) (*UserOperationResult, error) {
	op, opHash, err := c.GetUserOperationAndHashToSign(c.Signer.GetAddress(), callData)
	if err != nil {
		return nil, err
	}

	signature, err := c.Signer.SignUserOperationHash(*opHash)
	if err != nil {
		return nil, err
	}

	op.Signature = signature

	return c.SendSignedUserOperation(op, waitForReceipt)
}

func (c *Client) GetUserOperationReceipt(result *UserOperationResult) (*UserOperationReceipt, error) {
	return c.BundlerClient.GetUserOperationReceipt(result.UserOperationHash, c.ReceiptPollingDelay, c.ReceiptPollingRetries)
}

func (c *Client) GetSmartAccountSigner(address common.Address, pk *ecdsa.PrivateKey) (types.AccountSigner, error) {
	return account.NewSmartAccountPrivateKeySigner(c.RpcClients.Network, address, pk)
}
