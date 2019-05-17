package validate

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// for now validator only counts the transactions
type validator struct {
	txCount int
}

func newValidator() *validator {
	return &validator{txCount: 0}
}

func (v *validator) validate(tx gomel.Tx) {
	v.txCount++
}
