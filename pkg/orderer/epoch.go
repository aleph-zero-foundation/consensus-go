package orderer

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// epoch is a wrapper around a triple (adder, dag, extender) that is processing units from a particular epoch.
// Units/Preunits can be added to the epoch by directly accessing methods of adder.
// extender produces timing rounds on the provided output channel.
type epoch struct {
	id       gomel.EpochID
	adder    gomel.Adder
	dag      gomel.Dag
	extender *linear.ExtenderService
	rs       gomel.RandomSource
	finished chan bool
	log      zerolog.Logger
}

func newEpoch(id gomel.EpochID, conf config.Config, syncer gomel.Syncer, rsf gomel.RandomSourceFactory, alert gomel.Alerter, unitBelt chan<- gomel.Unit, output chan<- []gomel.Unit, log zerolog.Logger) *epoch {
	log = log.With().Uint32(logging.Epoch, uint32(id)).Logger()
	dg := dag.New(conf, id)
	adr := adder.New(dg, conf, syncer, alert, log)
	rs := rsf.NewRandomSource(dg)
	ext := linear.NewExtenderService(dg, rs, conf, output, log)

	dg.AfterInsert(func(_ gomel.Unit) { ext.Notify() })
	dg.AfterInsert(func(u gomel.Unit) {
		log.Debug().Uint16(logging.Creator, u.Creator()).Uint32(logging.Epoch, uint32(u.EpochID())).Int(logging.Height, u.Height()).Int(logging.Level, u.Level()).Msg(logging.BeforeSendingUnitToCreator)
		if u.Creator() != conf.Pid { // don't put our own units on the unit belt, creator already knows about them.
			unitBelt <- u
		}
	})

	log.Log().Msg(logging.NewEpoch)
	return &epoch{
		id:       id,
		adder:    adr,
		dag:      dg,
		extender: ext,
		rs:       rs,
		finished: make(chan bool),
		log:      log,
	}
}

// Close stops all the workers inside this epoch.
func (ep *epoch) Close() {
	ep.adder.Close()
	ep.extender.Close()
	ep.log.Log().Msg(logging.EpochEnd)
}

func (ep *epoch) unitsAbove(heights []int) []gomel.Unit {
	return ep.dag.UnitsAbove(heights)
}

func (ep *epoch) allUnits() []gomel.Unit {
	return ep.dag.UnitsAbove(nil)
}

// IsFinished checks if this epoch is still interested in accepting new units.
func (ep *epoch) IsFinished() bool {
	select {
	case <-ep.finished:
		return true
	default:
		return false
	}
}

func (ep *epoch) Finish() {
	close(ep.finished)
}
