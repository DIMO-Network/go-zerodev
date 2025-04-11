package types

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
)

type RPCClient interface {
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
	Close()
}

type AccountSigner interface {
	GetAddress() common.Address
	SignMessage(message []byte) ([]byte, error)
	SignTypedData(typedData *signer.TypedData) ([]byte, error)
	SignHash(hash common.Hash) ([]byte, error)
	SignUserOperationHash(hash common.Hash) ([]byte, error)
}
