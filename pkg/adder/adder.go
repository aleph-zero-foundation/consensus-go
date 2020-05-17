package adder

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
)

// adder is a buffer zone where preunits wait to be added to dag. A preunit with
// missing parents is waiting until all the parents are available. Then it's considered
// 'ready' and added to per-pid channel, from where it's picked by the worker.
// Adding a unit consists of:
// a) DecodeParents
// b) BuildUnit
// c) Check
// d) Insert
type adder struct {
	dag         gomel.Dag
	alert       gomel.Alerter
	conf        config.Config
	syncer      gomel.Syncer
	ready       []chan *waitingPreunit
	waiting     map[gomel.Hash]*waitingPreunit
	waitingByID map[uint64]*waitingPreunit
	missing     map[uint64]*missingPreunit
	finished    chan struct{}
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
		finished:    make(chan struct{}),
		log:         log.With().Int(lg.Service, lg.AdderService).Logger(),
	}
	for i := range ad.ready {
		if uint16(i) == ad.conf.Pid {
			continue
		}
		ad.ready[i] = make(chan *waitingPreunit, conf.EpochLength)
		ad.wg.Add(1)
		go func(ch chan *waitingPreunit) {
			defer ad.wg.Done()
			for {
				select {
				case wp := <-ch:
					ad.handleReady(wp)
				case <-ad.finished:
					return
				}
			}
		}(ad.ready[i])
	}
	ad.log.Info().Msg(lg.ServiceStarted)
	return ad
}

// Close stops the adder.
func (ad *adder) Close() {
	close(ad.finished)
	ad.wg.Wait()
	ad.log.Info().Msg(lg.ServiceStopped)
}

// AddPreunits checks basic correctness of a slice of preunits and then adds correct ones to the buffer zone.
// Returned slice can have the following members:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddPreunits(source uint16, preunits ...gomel.Preunit) []error {
	ad.log.Debug().Int(lg.Size, len(preunits)).Uint16(lg.PID, source).Msg(lg.AddPreunits)
	var errors []error
	getErrors := func() []error {
		if errors == nil {
			errors = make([]error, len(preunits))
		}
		return errors
	}
	hashes := make([]*gomel.Hash, len(preunits))
	for i, pu := range preunits {
		hashes[i] = pu.Hash()
	}
	alreadyInDag := ad.dag.GetUnits(hashes)

	failed := make([]bool, len(preunits))
	for i, pu := range preunits {
		if alreadyInDag[i] == nil {
			err := ad.checkCorrectness(pu)
			if err != nil {
				getErrors()[i] = err
				failed[i] = true
			}
		} else {
			getErrors()[i] = gomel.NewDuplicateUnit(alreadyInDag[i])
			failed[i] = true
		}
	}

	ad.mx.Lock()
	defer ad.mx.Unlock()
	for i, pu := range preunits {
		if !failed[i] {
			getErrors()[i] = ad.addToWaiting(pu, source)
		}
	}
	return errors
}

// addPreunit as a waitingPreunit to the buffer zone.
// This method must be called under mutex!
func (ad *adder) addToWaiting(pu gomel.Preunit, source uint16) error {
	if wp, ok := ad.waiting[*pu.Hash()]; ok {
		return gomel.NewDuplicatePreunit(wp.pu)
	}
	if u := ad.dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
	id := gomel.UnitID(pu)
	if fork, ok := ad.waitingByID[id]; ok {
		ad.log.Warn().Int(lg.Height, pu.Height()).Uint16(lg.Creator, pu.Creator()).Uint16(lg.PID, source).Msg(lg.ForkDetected)
		ad.alert.NewFork(pu, fork.pu)
	}
	wp := &waitingPreunit{pu: pu, id: id, source: source}
	ad.waiting[*pu.Hash()] = wp
	ad.waitingByID[id] = wp
	maxHeights := ad.checkParents(wp)
	ad.checkIfMissing(wp)
	if wp.missingParents > 0 {
		ad.log.Debug().Int(lg.Height, wp.pu.Height()).Uint16(lg.Creator, wp.pu.Creator()).Uint16(lg.PID, wp.source).Int(lg.Size, wp.missingParents).Msg(lg.UnknownParents)
		ad.fetchMissing(wp, maxHeights)
		return gomel.NewUnknownParents(wp.missingParents)
	}
	ad.sendIfReady(wp)
	return nil
}

// sendIfReady checks if a waitingPreunit is ready (has no waiting or missing parents).
// If yes, the preunit is sent to the channel corresponding to its dedicated worker.
// Atomic flag prevents send on a closed channel after Stop().
func (ad *adder) sendIfReady(wp *waitingPreunit) {
	if wp.waitingParents == 0 && wp.missingParents == 0 {
		ad.ready[wp.pu.Creator()] <- wp
	}
}

// handleReady takes a waitingPreunit that is ready and adds it to the dag.
func (ad *adder) handleReady(wp *waitingPreunit) {
	defer ad.remove(wp)
	log := ad.log.With().Int(lg.Height, wp.pu.Height()).Uint16(lg.Creator, wp.pu.Creator()).Uint16(lg.PID, wp.source).Logger()
	log.Debug().Msg(lg.PreunitReady)

	// 1. Decode Parents
	parents, err := ad.dag.DecodeParents(wp.pu)
	if err != nil {
		if e, ok := err.(*gomel.AmbiguousParents); ok {
			parents = make([]gomel.Unit, 0, len(e.Units))
			for _, us := range e.Units {
				parent, err := ad.alert.Disambiguate(us, wp.pu)
				err = ad.alert.ResolveMissingCommitment(err, wp.pu, wp.source)
				if err != nil {
					break
				}
				parents = append(parents, parent)
			}
		}
		if err != nil {
			log.Error().Str("where", "DecodeParents").Msg(err.Error())
			wp.failed = true
			return
		}
	}
	if *gomel.CombineHashes(gomel.ToHashes(parents)) != wp.pu.View().ControlHash {
		wp.failed = true
		ad.log.Warn().Bytes(lg.ControlHash, wp.pu.View().ControlHash[:]).Uint16(lg.PID, wp.source).Ints(lg.Height, wp.pu.View().Heights).Msg(lg.InvalidControlHash)
		ad.handleInvalidControlHash(wp.source, wp.pu, parents)
		return
	}

	// 2. Build Unit
	freeUnit := ad.dag.BuildUnit(wp.pu, parents)

	// 3. Check
	ad.alert.Lock(freeUnit.Creator())
	defer ad.alert.Unlock(freeUnit.Creator())
	err = ad.dag.Check(freeUnit)
	err = ad.alert.ResolveMissingCommitment(err, freeUnit, wp.source)
	if err != nil {
		log.Error().Str("where", "Check").Msg(err.Error())
		wp.failed = true
		return
	}

	// 4. Insert
	ad.dag.Insert(freeUnit)

	log.Debug().Int(lg.Level, freeUnit.Level()).Msg(lg.UnitAdded)
}

func (ad *adder) handleInvalidControlHash(sourcePID uint16, witness gomel.Preunit, parentCandidates []gomel.Unit) {
	ids := make([]uint64, 0, len(witness.View().Heights))
	for pid, height := range witness.View().Heights {
		ids = append(ids, gomel.ID(height, uint16(pid), witness.EpochID()))
	}
	// this should trigger download of all parents, including some that are witnesses of forks,
	// and start an alert while they are added
	ad.syncer.RequestFetch(sourcePID, ids)
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
