package order

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
)

type epoch struct {
	id       gomel.EpochID
	adder    gomel.Adder
	dag      gomel.Dag
	extender *linear.Extender
}

func newEpoch(id gomel.EpochID, conf config.Config, syncer gomel.Syncer, rsf gomel.RandomSourceFactory, alert gomel.Alerter, unitBelt chan<- gomel.Unit, output chan<- []gomel.Unit, log zerolog.Logger) *epoch {
	dg := dag.New(conf, id)
	adr := adder.New(dg, conf, syncer, alert, log)
	rs := rsf.NewRandomSource(dg)
	ext := linear.NewExtender(dg, rs, conf, output, log)
	dg.AfterInsert(func(_ gomel.Unit) { ext.Notify() })
	dg.AfterInsert(func(u gomel.Unit) { unitBelt <- u })
	return &epoch{
		id:       id,
		adder:    adr,
		dag:      dg,
		extender: ext,
	}
}

func (ep *epoch) close() {
	ep.adder.Close()
	ep.extender.Close()
}

func (ep *epoch) addPreunits(source uint16, preunits ...gomel.Preunit) {
	ep.adder.AddPreunits(source, preunits...)
}
