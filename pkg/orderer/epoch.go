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

type epoch struct {
	id       gomel.EpochID
	adder    gomel.Adder
	dag      gomel.Dag
	extender *linear.Extender
	rs       gomel.RandomSource
	log      zerolog.Logger
}

func newEpoch(id gomel.EpochID, conf config.Config, syncer gomel.Syncer, rsf gomel.RandomSourceFactory, alert gomel.Alerter, unitBelt chan<- gomel.Unit, output chan<- []gomel.Unit, log zerolog.Logger) *epoch {
	log = log.With().Uint32(logging.Epoch, uint32(id)).Logger()
	dg := dag.New(conf, id)
	adr := adder.New(dg, conf, syncer, alert, log)
	rs := rsf.NewRandomSource(dg)
	ext := linear.NewExtender(dg, rs, conf, output, log)
	dg.AfterInsert(func(_ gomel.Unit) { ext.Notify() })
	dg.AfterInsert(func(u gomel.Unit) {
		if u.Creator() != conf.Pid { // don't put our own units on the unit belt, creator already knows about them.
			unitBelt <- u
		}
	})
	log.Info().Msg(logging.NewEpoch)
	return &epoch{
		id:       id,
		adder:    adr,
		dag:      dg,
		extender: ext,
		rs:       rs,
		log:      log,
	}
}

func (ep *epoch) close() {
	ep.adder.Close()
	ep.extender.Close()
	ep.log.Info().Msg(logging.EpochEnd)
}

func (ep *epoch) unitsAbove(heights []int) []gomel.Unit {
	return ep.dag.UnitsAbove(heights)
}

func (ep *epoch) allUnits() []gomel.Unit {
	return ep.dag.UnitsAbove(nil)
}
