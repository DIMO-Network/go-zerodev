package zerodev

import (
	"crypto/ecdsa"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

type PrivateKeySigner struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

func NewPrivateKeySigner(privateKey *ecdsa.PrivateKey) (*PrivateKeySigner, error) {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	return &PrivateKeySigner{
		PrivateKey: privateKey,
		Address:    crypto.PubkeyToAddress(*publicKeyECDSA),
	}, nil
}

func (s *PrivateKeySigner) GetAddress() common.Address {
	return s.Address
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
