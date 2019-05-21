package validate

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type validator struct {
	userBalance map[string]uint32
}

func newValidator(userBalance map[string]uint32) *validator {
	return &validator{userBalance: userBalance}
}

func (v *validator) validate(tx gomel.Tx) {
	if v.userBalance[tx.Issuer] >= tx.Amount {
		v.userBalance[tx.Issuer] -= tx.Amount
		v.userBalance[tx.Receiver] += tx.Amount
	}
}
