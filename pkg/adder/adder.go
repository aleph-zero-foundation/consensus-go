package adder

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
)

const (
	channelLength = 32
)

// adder is a buffer zone where preunits wait to be added to dag. A preunit with
// missing parents is waiting until all the parents are available. Then it's considered
// 'ready' and added to per-pid channel, from where it's picked by the worker.
// Adding a unit consists of:
// a) DecodeParents
// b) BuildUnit
// c) Check
// d) Transform
// e) Insert
type adder struct {
	dag         gomel.Dag
	alert       gomel.Alerter
	conf        config.Config
	syncer      gomel.Syncer
	ready       []chan *waitingPreunit
	waiting     map[gomel.Hash]*waitingPreunit
	waitingByID map[uint64]*waitingPreunit
	missing     map[uint64]*missingPreunit
	active      bool
	rmx         sync.RWMutex
	mx          sync.Mutex
	wg          sync.WaitGroup
	log         zerolog.Logger
}

// New constructs a new adder.
func New(dag gomel.Dag, conf config.Config, syncer gomel.Syncer, alert gomel.Alerter, log zerolog.Logger) gomel.Adder {
	ad := &adder{
		dag:         dag,
		alert:       alert,
		conf:        conf,
		syncer:      syncer,
		ready:       make([]chan *waitingPreunit, dag.NProc()),
		waiting:     make(map[gomel.Hash]*waitingPreunit),
		waitingByID: make(map[uint64]*waitingPreunit),
		missing:     make(map[uint64]*missingPreunit),
		active:      true,
		log:         log,
	}
	for i := range ad.ready {
		if uint16(i) == ad.conf.Pid {
			continue
		}
		ad.ready[i] = make(chan *waitingPreunit, channelLength)
		ad.wg.Add(1)
		go func(ch chan *waitingPreunit) {
			defer ad.wg.Done()
			for wp := range ch {
				ad.handleReady(wp)
			}
		}(ad.ready[i])
	}
	ad.log.Info().Msg(logging.ServiceStarted)
	return ad
}

// Close stops the adder.
func (ad *adder) Close() {
	ad.rmx.Lock()
	ad.active = false
	ad.rmx.Unlock()
	for _, c := range ad.ready {
		close(c)
	}
	ad.wg.Wait()
	ad.log.Info().Msg(logging.ServiceStopped)
}

// AddPreunits checks basic correctness of a given slice of preunits and then adds them to the buffer zone.
// Does not block - this method returns when all preunits are added to the waiting preunits (or rejected due to signature or duplication).
// Returned AggregateError can have the following members:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents  - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddPreunits(source uint16, preunits ...gomel.Preunit) []error {
	ad.log.Debug().Int(logging.Size, len(preunits)).Uint16(logging.PID, source).Msg(logging.AddUnitsStarted)
	errors := make([]error, len(preunits))
	hashes := make([]*gomel.Hash, len(preunits))
	for i, pu := range preunits {
		hashes[i] = pu.Hash()
	}
	alreadyInDag := ad.dag.GetUnits(hashes)

	for i, pu := range preunits {
		if alreadyInDag[i] == nil {
			err := ad.checkCorrectness(pu)
			if err != nil {
				errors[i] = err
				preunits[i] = nil
			}
		} else {
			errors[i] = gomel.NewDuplicateUnit(alreadyInDag[i])
			preunits[i] = nil
		}
	}

	ad.mx.Lock()
	defer ad.mx.Unlock()
	for i, pu := range preunits {
		if pu == nil {
			continue
		}
		errors[i] = ad.addToWaiting(pu, source)
	}
	return errors
}

// sendIfReady checks if a waitingPreunit is ready (has no waiting or missing parents).
// If yes, the preunit is sent to the channel corresponding to its dedicated worker.
// Atomic flag prevents send on a closed channel after Stop().
func (ad *adder) sendIfReady(wp *waitingPreunit) {
	ad.rmx.RLock()
	defer ad.rmx.RUnlock()
	if wp.waitingParents == 0 && wp.missingParents == 0 && ad.active {
		ad.ready[wp.pu.Creator()] <- wp
		ad.log.Debug().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Msg(logging.PreunitReady)
	}
}

// handleReady takes a waitingPreunit that is ready and adds it to the dag.
func (ad *adder) handleReady(wp *waitingPreunit) {
	defer ad.remove(wp)
	log := ad.log.With().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Logger()
	log.Debug().Msg(logging.AddingStarted)

	// 1. Decode Parents
	parents, err := ad.dag.DecodeParents(wp.pu)
	if err != nil {
		parents, err = ad.handleDecodeError(err, wp)
		if err != nil {
			log.Error().Str("where", "DecodeParents").Msg(err.Error())
			wp.failed = true
			return
		}
	}

	// 2. Build Unit
	freeUnit := unit.FromPreunit(wp.pu, parents)

	// 3. Check
	ad.alert.Lock(freeUnit.Creator())
	defer ad.alert.Unlock(freeUnit.Creator())
	err = ad.handleCheckError(ad.dag.Check(freeUnit), freeUnit, wp.source)
	if err != nil {
		log.Error().Str("where", "Check").Msg(err.Error())
		wp.failed = true
		return
	}

	// 4. Insert
	ad.dag.Insert(freeUnit)

	log.Info().Msg(logging.UnitAdded)
}

// checkCorrectness checks very basic correctness of the given preunit: creator and signature.
func (ad *adder) checkCorrectness(pu gomel.Preunit) error {
	if pu.Creator() >= ad.dag.NProc() {
		return gomel.NewDataError("invalid creator")
	}
	if pu.EpochID() != ad.dag.EpochID() {
		return gomel.NewDataError(
			fmt.Sprintf("invalid EpochID - expected %d, but received %d instead", ad.dag.EpochID(), pu.EpochID()),
		)
	}
	if !ad.conf.PublicKeys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("invalid signature")
	}
	return nil
}

func (ad *adder) handleDecodeError(err error, wp *waitingPreunit) ([]gomel.Unit, error) {
	switch e := err.(type) {
	case *gomel.AmbiguousParents:
		parents := make([]gomel.Unit, 0, len(e.Units))
		for _, us := range e.Units {
			parent, err2 := ad.alert.Disambiguate(us, wp.pu)
			err2 = ad.alert.ResolveMissingCommitment(err2, wp.pu, wp.source)
			if err2 != nil {
				return nil, err2
			}
			parents = append(parents, parent)
		}
		if *gomel.CombineHashes(gomel.ToHashes(parents)) != wp.pu.View().ControlHash {
			return nil, gomel.NewDataError("wrong control hash")
		}
		return parents, nil
	default:
		return nil, err
	}
}

func (ad *adder) handleCheckError(err error, u gomel.Unit, source uint16) error {
	if err == nil {
		return nil
	}
	return ad.alert.ResolveMissingCommitment(err, u, source)
}
