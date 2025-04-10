package account

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
)

type mockRPCClient struct {
	callContextFunc func(ctx context.Context, result interface{}, method string, args ...interface{}) error
}

func (m *mockRPCClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	if m.callContextFunc != nil {
		return m.callContextFunc(ctx, result, method, args...)
	}
	return nil
}

func (m *mockRPCClient) Close() {}

func TestGetAccountMetadata(t *testing.T) {
	expectedAddress := common.HexToAddress("0xC81d8Fa063A7C73795C8455F6b766dd245D8F47A")

	tests := []struct {
		name          string
		mockResponse  func(ctx context.Context, result interface{}, method string, args ...interface{}) error
		expectedError error
		expectedData  *AccountMetadata
	}{
		{
			name: "valid_response",
			mockResponse: func(ctx context.Context, result interface{}, method string, args ...interface{}) error {
				expected := hexutil.MustDecode(`0x0f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000013882000000000000000000000000c81d8fa063a7c73795c8455f6b766dd245d8f47a0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000000064b65726e656c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005302e332e310000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000`)
				*result.(*hexutil.Bytes) = expected
				return nil
			},
			expectedError: nil,
			expectedData: &AccountMetadata{
				Fields:            [1]byte{0x0f},
				Name:              "Kernel",
				Version:           "0.3.1",
				ChainId:           big.NewInt(80002),
				VerifyingContract: expectedAddress,
				Extensions:        make([]*big.Int, 0),
			},
		},
		{
			name: "rpc_call_error",
			mockResponse: func(ctx context.Context, result interface{}, method string, args ...interface{}) error {
				return errors.New("rpc call failed")
			},
			expectedError: errors.New("rpc call failed"),
			expectedData:  nil,
		},
		{
			name: "invalid_abi_unpack",
			mockResponse: func(ctx context.Context, result interface{}, method string, args ...interface{}) error {
				*result.(*hexutil.Bytes) = []byte{0x00}
				return nil
			},
			expectedError: errors.New("abi: improperly formatted output: "),
			expectedData:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockRPCClient{
				callContextFunc: tt.mockResponse,
			}

			result, err := GetAccountMetadata(mockClient, expectedAddress)

			if tt.expectedError != nil {
				assert.ErrorContains(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedData, result)
		})
	}
}
