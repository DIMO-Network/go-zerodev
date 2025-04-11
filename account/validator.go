package account

import "github.com/ethereum/go-ethereum/common"

type Validator interface {
	GetType() []byte
	GetAddress() common.Address
	GetIdentifier() []byte
}

const (
	ValidatorTypeSudo       = "0x00"
	ValidatorTypeSecondary  = "0x01"
	ValidatorTypePermission = "0x02"
)

const (
	EcdsaValidatorAddress = "0x845ADb2C711129d4f3966735eD98a9F09fC4cE57"
)

type EcdsaValidator struct {
	Type    []byte
	Address common.Address
}

func NewEcdsaValidator() *EcdsaValidator {
	return &EcdsaValidator{
		Type:    common.FromHex(ValidatorTypeSecondary),
		Address: common.HexToAddress(EcdsaValidatorAddress),
	}
}

func (e *EcdsaValidator) GetType() []byte {
	return e.Type
}

func (e *EcdsaValidator) GetAddress() common.Address {
	return e.Address
}

func (e *EcdsaValidator) GetIdentifier() []byte {
	return append(e.Type, e.Address.Bytes()...)
}
