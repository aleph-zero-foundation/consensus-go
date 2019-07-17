package add

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

// Antichain adds an antichain of preunits to the dag and reports a composite error if it fails.
// It also returns whether it successfully added a prime unit.
func Antichain(dag gomel.Dag, randomSource gomel.RandomSource, preunits []gomel.Preunit, fallback gsync.Fallback, log zerolog.Logger) (bool, *AggregateError) {
	var wg sync.WaitGroup
	problem := &AggregateError{
		errs: make([]error, len(preunits)),
	}
	var primeAdded int32
	for i, preunit := range preunits {
		i := i
		wg.Add(1)
		dag.AddUnit(preunit, randomSource, func(pu gomel.Preunit, added gomel.Unit, err error) {
			if err != nil {
				switch e := err.(type) {
				case *gomel.DuplicateUnit:
					log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
				case *gomel.UnknownParents:
					log.Info().Int(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
					fallback.Run(pu)
					problem.errs[i] = err
				default:
					problem.errs[i] = err
				}
			} else {
				if gomel.Prime(added) {
					atomic.StoreInt32(&primeAdded, 1)
				}
			}
			wg.Done()
		})
	}
	wg.Wait()
	return primeAdded == 1, problem.Pruned(false)
}
