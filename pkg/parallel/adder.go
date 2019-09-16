package parallel

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type addRequest struct {
	id  int
	pu  gomel.Preunit
	wg  *sync.WaitGroup
	err *error
}

type adder struct {
	id    int
	sinks []chan<- addRequest
}

// AddUnit to the internal dag, use the provided logger for logging.
func (da *adder) AddUnit(pu gomel.Preunit) error {
	if int(pu.Creator()) >= len(da.sinks) {
		return gomel.NewDataError("invalid creator")
	}
	var wg sync.WaitGroup
	var err error
	wg.Add(1)
	da.sinks[pu.Creator()] <- addRequest{da.id, pu, &wg, &err}
	wg.Wait()
	return err
}

// AddAntichain of units to the internal dag, use the provided logger for logging.
func (da *adder) AddAntichain(preunits []gomel.Preunit) *gomel.AggregateError {
	var wg sync.WaitGroup
	result := make([]error, len(preunits))
	wg.Add(len(preunits))
	for i, pu := range preunits {
		if int(pu.Creator()) >= len(da.sinks) {
			result[i] = gomel.NewDataError("invalid creator")
			continue
		}
		da.sinks[pu.Creator()] <- addRequest{da.id, pu, &wg, &result[i]}
	}
	wg.Wait()
	return gomel.NewAggregateError(result)
}
