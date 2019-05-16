package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// TxValidator is a service for validating transactions
// unitSource is a channel from which the validator is consuming units
// for now it is only counting transactions
type TxValidator struct {
	unitSource <-chan gomel.Unit
	exitChan   chan struct{}
	txCount    int
}

// NewTxValidator is a constructor of tx_validator service
func NewTxValidator(unitSource chan gomel.Unit) *TxValidator {
	return &TxValidator{
		unitSource: unitSource,
		exitChan:   make(chan struct{}),
		txCount:    0,
	}
}

func (tv *TxValidator) validate(t gomel.Tx) {
	tv.txCount++
}

func (tv *TxValidator) main() {
	for {
		select {
		case u := <-tv.unitSource:
			for _, t := range u.Txs() {
				tv.validate(t)
			}
		case <-tv.exitChan:
			return
		}
	}
}

// Start is a function which starts the service
func (tv *TxValidator) Start() error {
	go tv.main()
	return nil
}

// Stop is the function that stops the service
func (tv *TxValidator) Stop() {
	close(tv.exitChan)
}
