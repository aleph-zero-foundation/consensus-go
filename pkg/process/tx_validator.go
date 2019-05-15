package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// TxValidator is a service for validating transactions
// unitSource is a channel from which the validator is consuming units
type TxValidator struct {
	unitSource  <-chan gomel.Unit
	exitChan    chan struct{}
	userBalance map[string]int
}

// NewTxValidator is a constructor of tx_validator service
func NewTxValidator(unitSource chan gomel.Unit) *TxValidator {
	return &TxValidator{
		unitSource:  unitSource,
		exitChan:    make(chan struct{}),
		userBalance: make(map[string]int),
	}
}

func validate(t gomel.Tx) {
	if userBalance[t.Issuer] >= t.Amount {
		userBalance[t.Issuer] -= t.Amount
		userBalance[t.Receiver] += t.Amont
	}
}

func (tv *TxValidator) main() {
	for {
		select {
		case u := <-tv.unitSource:
			for t := range u.Txs() {
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
