package add

import (
	"sync"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

// Antichain adds an antichain of preunits to the poset and reports a composite error if it fails.
// It also returns whether it successfully added a prime unit.
func Antichain(poset gomel.Poset, randomSource gomel.RandomSource, preunits []gomel.Preunit, fallback gsync.Fallback, log zerolog.Logger) (bool, *AggregateError) {
	var wg sync.WaitGroup
	problem := &AggregateError{
		errs: make([]error, len(preunits)),
	}
	primeAdded := false
	for i, preunit := range preunits {
		i := i
		wg.Add(1)
		poset.AddUnit(preunit, randomSource, func(pu gomel.Preunit, added gomel.Unit, err error) {
			if err != nil {
				switch e := err.(type) {
				case *gomel.DuplicateUnit:
					log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
				case *gomel.UnknownParent:
					log.Info().Int(logging.Creator, pu.Creator()).Msg(logging.UnknownParents)
					fallback.Run(pu)
					problem.errs[i] = err
				default:
					problem.errs[i] = err
				}
			} else {
				if gomel.Prime(added) {
					primeAdded = true
				}
			}
			wg.Done()
		})
	}
	wg.Wait()
	return primeAdded, problem.Pruned(false)
}
