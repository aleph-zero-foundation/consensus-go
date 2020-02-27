package logging

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// AddingErrors logs information about errors from AddPreunits to provided logger.
func AddingErrors(errors []error, log zerolog.Logger) {
	if !(len(errors) > 0) {
		return
	}
	ok, units, preunits := 0, 0, 0
	for _, err := range errors {
		if err == nil {
			ok++
			continue
		}
		switch e := err.(type) {
		case *gomel.DuplicateUnit:
			units++
		case *gomel.DuplicatePreunit:
			preunits++
		case *gomel.UnknownParents:
			log.Info().Int(Size, e.Amount).Msg(UnknownParents)
		default:
			log.Error().Str("where", "adding").Msg(err.Error())
		}

	}
	if units > 0 {
		log.Info().Int(Size, units).Msg(DuplicatedUnits)
	}
	if preunits > 0 {
		log.Info().Int(Size, preunits).Msg(DuplicatedPreunits)
	}
	if ok > 0 {
		log.Info().Int(Size, ok).Msg(SuccesfulAdd)
	}
}
