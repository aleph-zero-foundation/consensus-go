package orderer

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creator"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type epoch struct {
	id       gomel.EpochID
	adder    gomel.Adder
	dag      gomel.Dag
	extender *linear.ExtenderService
	rs       gomel.RandomSource
	proxy    chan []gomel.Unit
	finished chan bool
	wait     sync.WaitGroup
	log      zerolog.Logger
}

func newEpoch(id gomel.EpochID, conf config.Config, syncer gomel.Syncer, rsf gomel.RandomSourceFactory, alert gomel.Alerter, unitBelt chan<- gomel.Unit, output chan<- []gomel.Unit, log zerolog.Logger) *epoch {
	log = log.With().Uint32(logging.Epoch, uint32(id)).Logger()
	dg := dag.New(conf, id)
	adr := adder.New(dg, conf, syncer, alert, log)
	rs := rsf.NewRandomSource(dg)

	proxy := make(chan []gomel.Unit, 1)
	ext := linear.NewExtenderService(dg, rs, conf, proxy, log)

	dg.AfterInsert(func(_ gomel.Unit) { ext.Notify() })
	dg.AfterInsert(func(u gomel.Unit) {
		if u.Creator() != conf.Pid { // don't put our own units on the unit belt, creator already knows about them.
			unitBelt <- u
		}
	})
	log.Info().Msg(logging.NewEpoch)
	epoch := &epoch{
		id:       id,
		adder:    adr,
		dag:      dg,
		extender: ext,
		rs:       rs,
		finished: make(chan bool),
		proxy:    proxy,
		log:      log,
	}

	epoch.wait.Add(1)
	go func() {
		defer epoch.wait.Done()
		for round := range proxy {
			timingUnit := round[len(round)-1]
			if timingUnit.Level() >= conf.EpochLength && creator.EpochProof(timingUnit, conf.WTKey) {
				epoch.finish()
			}
			output <- round
		}
	}()

	return epoch
}

func (ep *epoch) Close() {
	ep.adder.Close()
	ep.extender.Close()
	close(ep.proxy)
	ep.wait.Wait()
	ep.log.Info().Msg(logging.EpochEnd)
}

func (ep *epoch) unitsAbove(heights []int) []gomel.Unit {
	return ep.dag.UnitsAbove(heights)
}

func (ep *epoch) allUnits() []gomel.Unit {
	return ep.dag.UnitsAbove(nil)
}

func (ep *epoch) IsFinished() bool {
	select {
	case <-ep.finished:
		return true
	default:
		return false
	}
}

func (ep *epoch) finish() {
	close(ep.finished)
}
