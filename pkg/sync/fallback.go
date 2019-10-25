package sync

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// FetchData that is neccessary to add the preunit to the dag. Fetch the data from process pid.
type FetchData func(pu gomel.Preunit, pid uint16) error

// Fallback can find out information about an unknown preunit.
type Fallback interface {
	// Resolve requests information about a problematic preunit.
	Resolve(gomel.Preunit)
}

type def struct {
	log zerolog.Logger
}

func (d *def) Resolve(pu gomel.Preunit) {
	d.log.Error().Uint16(logging.Creator, pu.Creator()).Str(logging.Hash, gomel.Nickname(pu)).Msg(logging.FallbackUsed)
}

// DefaultFallback returns a fallback that does nothing and logs an error to provided with logger.
func DefaultFallback(log zerolog.Logger) Fallback {
	return &def{log}
}
