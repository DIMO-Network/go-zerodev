package account

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

var (
	bytes32, _ = abi.NewType("bytes32", "", nil)
)

type AccountPrivateKeySigner struct {
	Address         common.Address
	PrivateKey      *ecdsa.PrivateKey
	AccountMetadata *AccountMetadata
}

func NewAccountPrivateKeySigner(address common.Address, privateKey *ecdsa.PrivateKey, accountMetadata *AccountMetadata) *AccountPrivateKeySigner {
	return &AccountPrivateKeySigner{
		Address:         address,
		PrivateKey:      privateKey,
		AccountMetadata: accountMetadata,
	}
}

func (s *AccountPrivateKeySigner) SignMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(message)
	return s.SignHash(hash)
}

func (s *AccountPrivateKeySigner) SignTypedData(typedData *signer.TypedData) ([]byte, error) {
	hash, _, err := signer.TypedDataAndHash(*typedData)
	if err != nil {
		return nil, err
	}

	accountTypedData, err := s.GetAccountTypedData()
	if err != nil {
		return nil, err
	}

	domainSeparator, err := accountTypedData.HashStruct("EIP712Domain", accountTypedData.Domain.Map())
	if err != nil {
		return nil, err
	}

	wrappedHash, err := s.WrapHash(common.BytesToHash(hash))
	if err != nil {
		return nil, err
	}

	rawData := fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(wrappedHash))
	finalHash := crypto.Keccak256Hash([]byte(rawData))

	// TODO: add validator here 1B + 20B

	return s.SignHash(finalHash)
}

func (s *AccountPrivateKeySigner) SignHash(hash common.Hash) ([]byte, error) {
	signature, err := crypto.Sign(hash.Bytes(), s.PrivateKey)

	if err != nil {
		return nil, err
	}
	signature[64] += 27
	return signature, nil
}

func (s *AccountPrivateKeySigner) WrapHash(hash common.Hash) ([]byte, error) {
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

func (s *AccountPrivateKeySigner) GetAccountTypedData() (*signer.TypedData, error) {
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
