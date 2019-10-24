// Package add implements functions for adding units to the dag in ways appropriate for various synchronization methods.
package add

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Unit adds a preunit to the dag and returns whether everything went fine.
func Unit(dag gomel.Dag, adder gomel.Adder, pu gomel.Preunit, where string, log zerolog.Logger) bool {
	return handleError(adder.AddUnit(pu, dag), pu, where, log)
}

// Chunk adds slice of antichains to the dag and returns whether everything went fine.
func Chunk(dag gomel.Dag, adder gomel.Adder, antichains [][]gomel.Preunit, where string, log zerolog.Logger) bool {
	success := true
	for _, antichain := range antichains {
		aggErr := adder.AddAntichain(antichain, dag)
		for i, err := range aggErr.Errors() {
			if !handleError(err, antichain[i], where, log) {
				success = false
			}
		}
	}
	return success
}

// handleError abstracts error processing for both above function. Returns false on serious errors.
func handleError(err error, pu gomel.Preunit, where string, log zerolog.Logger) bool {
	if err != nil {
		switch e := err.(type) {
		case *gomel.DuplicateUnit:
			log.Info().Uint16(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
		case *gomel.UnknownParents:
			log.Info().Uint16(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
			if fallback != nil {
				fallback.Resolve(pu)
			}
		case *gomel.MissingDataError:
			log.Info().Uint16(logging.Creator, pu.Creator()).Msg(logging.MissingDataError)
			if fetchData != nil {
				if err2 := fetchData(pu, peer); err2 != nil {
					log.Error().Str("addUnit", where+".fetchData").Msg(err2.Error())
					return false
				}
			}
		default:
			log.Error().Str("addUnit", where).Msg(err.Error())
			return false
		}
	}
	return true
}
