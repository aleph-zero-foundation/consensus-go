package sync

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Fallback can find out information about an unknown preunit.
type Fallback interface {
	// FindOut requests information about a problematic preunit.
	FindOut(gomel.Preunit)
}

type def struct {
	log zerolog.Logger
}

func (d *def) FindOut(pu gomel.Preunit) {
	d.log.Error().Uint16(logging.Creator, pu.Creator()).Str(logging.Hash, gomel.Nickname(pu)).Msg(logging.FallbackUsed)
}

// DefaultFallback returns a fallback that does nothing and logs an error to provided with logger.
func DefaultFallback(log zerolog.Logger) Fallback {
	return &def{log}
}
