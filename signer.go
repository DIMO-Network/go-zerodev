package zerodev

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/crypto"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

type Signer interface {
	SignMessage(message []byte) ([]byte, error)
	SignTypedData(typedData *signer.TypedData) ([]byte, error)
	SignHash(hash common.Hash) ([]byte, error)
	SignUserOperationHash(hash common.Hash) ([]byte, error)
}

type PrivateKeySigner struct {
	PrivateKey *ecdsa.PrivateKey
}

func NewPrivateKeySigner(privateKey *ecdsa.PrivateKey) Signer {
	return &PrivateKeySigner{
		PrivateKey: privateKey,
	}
}

func (s *PrivateKeySigner) SignMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(message)
	return s.SignHash(hash)
}

func (s *PrivateKeySigner) SignTypedData(typedData *signer.TypedData) ([]byte, error) {
	hash, _, err := signer.TypedDataAndHash(*typedData)
	if err != nil {
		return nil, err
	}

	return s.SignHash(common.BytesToHash(hash))
}

func (s *PrivateKeySigner) SignHash(hash common.Hash) ([]byte, error) {
	signature, err := crypto.Sign(hash.Bytes(), s.PrivateKey)

	if err != nil {
		return nil, err
	}
	signature[64] += 27
	return signature, nil
}

func (s *PrivateKeySigner) SignUserOperationHash(hash common.Hash) ([]byte, error) {
	return s.SignHash(hash)
}
