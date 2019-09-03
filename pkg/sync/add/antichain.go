// Package add implements functions for adding units to the dag in ways appropriate for various synchronisation methods.
package add

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

func handleFailure(errorAddr *error, fallback gsync.Fallback, log zerolog.Logger) gomel.Callback {
	return func(pu gomel.Preunit, added gomel.Unit, err error) {
		if err != nil {
			switch e := err.(type) {
			case *gomel.DuplicateUnit:
				log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
			case *gomel.UnknownParents:
				log.Info().Int(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
				fallback.Run(pu)
				*errorAddr = err
			default:
				*errorAddr = err
			}
		}
	}
}

// Unit adds a preunit to the dag and returns an error if it fails.
func Unit(dag gomel.Dag, randomSource gomel.RandomSource, preunit gomel.Preunit, callback gomel.Callback, fallback gsync.Fallback, log zerolog.Logger) error {
	var wg sync.WaitGroup
	var err error
	wg.Add(1)
	dag.AddUnit(preunit, randomSource, func(p gomel.Preunit, u gomel.Unit, e error) {
		defer wg.Done()
		handleFailure(&err, fallback, log)(p, u, e)
		callback(p, u, e)
	})
	wg.Wait()
	return err
}

// Antichain adds an antichain of preunits to the dag and reports a composite error if it fails.
func Antichain(dag gomel.Dag, randomSource gomel.RandomSource, preunits []gomel.Preunit, callback gomel.Callback, fallback gsync.Fallback, log zerolog.Logger) *AggregateError {
	var wg sync.WaitGroup
	problem := &AggregateError{
		errs: make([]error, len(preunits)),
	}
	for i, preunit := range preunits {
		wg.Add(1)
		dag.AddUnit(preunit, randomSource, func(p gomel.Preunit, u gomel.Unit, e error) {
			defer wg.Done()
			handleFailure(&problem.errs[i], fallback, log)(p, u, e)
			callback(p, u, e)
		})
	}
	wg.Wait()
	return problem.Pruned(false)
}
