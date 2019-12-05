// Package add implements functions for adding units to the dag in ways appropriate for various synchronization methods.
package add

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Unit adds a preunit to the dag and returns whether everything went fine.
func Unit(adder gomel.Adder, pu gomel.Preunit, source uint16, where string, log zerolog.Logger) bool {
	return handleError(adder.AddUnit(pu, source), pu, where, log)
}

// Chunk adds a slice of preunits to the dag and returns whether everything went fine.
func Chunk(adder gomel.Adder, preunits []gomel.Preunit, source uint16, where string, log zerolog.Logger) bool {
	if len(preunits) == 0 {
		return true
	}
	success := true
	aggErr := adder.AddUnits(preunits, source)
	for i, err := range aggErr.Errors() {
		if !handleError(err, preunits[i], where, log) {
			success = false
		}
	}
	return success
}

// handleError abstracts error processing for both above function. Returns false on serious errors.
func handleError(err error, pu gomel.Preunit, where string, log zerolog.Logger) bool {
	if err != nil {
		switch e := err.(type) {
		case *gomel.DuplicateUnit:
			log.Info().Uint16(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicateUnit)
		case *gomel.DuplicatePreunit:
			log.Info().Uint16(logging.Creator, e.Pu.Creator()).Int(logging.Height, e.Pu.Height()).Msg(logging.DuplicatePreunit)
		case *gomel.UnknownParents:
			log.Info().Uint16(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
		default:
			log.Error().Str("addUnit", where).Msg(err.Error())
			return false
		}
	}
	return true
}
