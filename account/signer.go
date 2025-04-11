package account

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/DIMO-Network/go-zerodev/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

var (
	bytes32, _ = abi.NewType("bytes32", "", nil)
)

type SmartAccountPrivateKeySigner struct {
	Client          types.RPCClient
	Address         common.Address
	PrivateKey      *ecdsa.PrivateKey
	Validator       Validator
	AccountMetadata *AccountMetadata
}

func NewSmartAccountPrivateKeySigner(client types.RPCClient, address common.Address, privateKey *ecdsa.PrivateKey) (*SmartAccountPrivateKeySigner, error) {
	return &SmartAccountPrivateKeySigner{
		Client:     client,
		Address:    address,
		PrivateKey: privateKey,
		Validator:  NewEcdsaValidator(),
	}, nil
}

func (s *SmartAccountPrivateKeySigner) GetAddress() common.Address {
	return s.Address
}

func (s *SmartAccountPrivateKeySigner) SignMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(message)
	return s.SignHash(hash)
}

func (s *SmartAccountPrivateKeySigner) SignTypedData(typedData *signer.TypedData) ([]byte, error) {
	hash, _, err := signer.TypedDataAndHash(*typedData)
	if err != nil {
		return nil, err
	}

	return s.SignHash(common.BytesToHash(hash))
}

func (s *SmartAccountPrivateKeySigner) SignHash(hash common.Hash) ([]byte, error) {
	accountTypedData, err := s.getAccountTypedData()
	if err != nil {
		return nil, err
	}

	domainSeparator, err := accountTypedData.HashStruct("EIP712Domain", accountTypedData.Domain.Map())
	if err != nil {
		return nil, err
	}

	wrappedHash, err := s.kernelHashWrap(hash)
	if err != nil {
		return nil, err
	}

	rawData := fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(wrappedHash))
	finalHash := crypto.Keccak256Hash([]byte(rawData))

	signature, err := s.signHashBase(finalHash)

	return append(s.Validator.GetIdentifier(), signature...), nil
}

func (s *SmartAccountPrivateKeySigner) SignUserOperationHash(hash common.Hash) ([]byte, error) {
	return s.signHashBase(hash)
}

func (s *SmartAccountPrivateKeySigner) signHashBase(hash common.Hash) ([]byte, error) {
	signature, err := crypto.Sign(hash.Bytes(), s.PrivateKey)
	if err != nil {
		return nil, err
	}
	signature[64] += 27

	return signature, nil
}

func (s *SmartAccountPrivateKeySigner) kernelHashWrap(hash common.Hash) ([]byte, error) {
	args := abi.Arguments{
		{Type: bytes32},
		{Type: bytes32},
	}

	packed, err := args.Pack(crypto.Keccak256Hash([]byte("Kernel(bytes32 hash)")), hash)
	if err != nil {
		return nil, err
	}

	return crypto.Keccak256(packed), nil
}

func (s *SmartAccountPrivateKeySigner) getAccountTypedData() (*signer.TypedData, error) {
	if s.AccountMetadata == nil {
		accountMetadata, err := GetAccountMetadata(s.Client, s.Address)
		if err != nil {
			return nil, err
		}

		s.AccountMetadata = accountMetadata
	}

	return &signer.TypedData{
		Types: signer.Types{
			"EIP712Domain": []signer.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
		},
		Domain: signer.TypedDataDomain{
			Name:              s.AccountMetadata.Name,
			Version:           s.AccountMetadata.Version,
			ChainId:           math.NewHexOrDecimal256(s.AccountMetadata.ChainId.Int64()),
			VerifyingContract: s.AccountMetadata.VerifyingContract.String(),
		},
	}, nil
}
