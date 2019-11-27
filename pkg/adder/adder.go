package adder

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
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
	keys        []gomel.PublicKey
	ready       []chan *waitingPreunit
	waiting     map[gomel.Hash]*waitingPreunit
	waitingByID map[uint64]*waitingPreunit
	missing     map[uint64]*missingPreunit
	mx          sync.Mutex
	wg          sync.WaitGroup
	quit        int64
	log         zerolog.Logger
}

// New constructs a new adder that uses the given set of public keys to verify correctness of incoming preunits.
// Returns twice the same object implementing both gomel.Adder and gomel.Service.
// Passing nil as keys disables signature checking.
func New(dag gomel.Dag, alert gomel.Alerter, keys []gomel.PublicKey, log zerolog.Logger) (gomel.Adder, gomel.Service) {
	ready := make([]chan *waitingPreunit, dag.NProc())
	for i := range ready {
		ready[i] = make(chan *waitingPreunit, 32)
	}
	ad := &adder{
		dag:         dag,
		alert:       alert,
		keys:        keys,
		ready:       ready,
		waiting:     make(map[gomel.Hash]*waitingPreunit),
		waitingByID: make(map[uint64]*waitingPreunit),
		missing:     make(map[uint64]*missingPreunit),
		log:         log,
	}
	return ad, ad
}

// AddUnit checks basic correctness of a preunit and then adds it to the buffer zone.
// Does not block - this method returns when the preunit is added to the waiting preunits.
// The returned error can be:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents  - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddUnit(pu gomel.Preunit, source uint16) error {
	ad.log.Debug().Int(logging.Height, pu.Height()).Uint16(logging.Creator, pu.Creator()).Uint16(logging.PID, source).Msg(logging.AddUnitStarted)
	if u := ad.dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
	err := ad.checkCorrectness(pu)
	if err != nil {
		return err
	}
	ad.mx.Lock()
	defer ad.mx.Unlock()
	return ad.addToWaiting(pu, source)
}

// AddUnits checks basic correctness of a given slice of preunits and then adds them to the buffer zone.
// Does not block - this method returns when all preunits are added to the waiting preunits (or rejected due to signature or duplication).
// Returned AggregateError can have the following members:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents  - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddUnits(preunits []gomel.Preunit, source uint16) *gomel.AggregateError {
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
	return gomel.NewAggregateError(errors)
}

// Start the adding workers.
func (ad *adder) Start() error {
	ad.wg.Add(int(ad.dag.NProc()))
	for i := range ad.ready {
		go func(i int) {
			defer ad.wg.Done()
			for wp := range ad.ready[i] {
				ad.handleReady(wp)
			}
		}(i)
	}
	ad.log.Info().Msg(logging.ServiceStarted)
	return nil
}

// Stop the adding workers.
func (ad *adder) Stop() {
	atomic.StoreInt64(&ad.quit, 1)
	for _, c := range ad.ready {
		close(c)
	}
	ad.wg.Wait()
	ad.log.Info().Msg(logging.ServiceStopped)
}

// sendIfReady checks if a waitingPreunit is ready (has no waiting or missing parents).
// If yes, the preunit is sent to the channel corresponding to its dedicated worker.
// Atomic flag prevents send on a closed channel after Stop().
func (ad *adder) sendIfReady(wp *waitingPreunit) {
	if wp.waitingParents == 0 && wp.missingParents == 0 && atomic.LoadInt64(&ad.quit) == 0 {
		ad.log.Debug().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Msg(logging.PreunitReady)
		ad.ready[wp.pu.Creator()] <- wp
	}
}

// handleReady takes a waitingPreunit that is ready and adds it to the dag.
func (ad *adder) handleReady(wp *waitingPreunit) {
	defer ad.remove(wp)
	log := ad.log.With().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Logger()
	log.Debug().Msg(logging.AddingStarted)
	parents, err := ad.dag.DecodeParents(wp.pu)
	if err != nil {
		switch e := err.(type) {
		case *gomel.AmbiguousParents:
			parents = make([]gomel.Unit, 0, len(e.Units))
			for _, us := range e.Units {
				parent, err2 := ad.alert.Disambiguate(us, wp.pu)
				err2 = ad.alert.ResolveMissingCommitment(err2, wp.pu, wp.source)
				if err2 != nil {
					log.Error().Str("where", "DecodeParents.Disambiguate").Msg(err2.Error())
					wp.failed = true
					return
				}
				parents = append(parents, parent)
			}
			if *gomel.CombineHashes(gomel.ToHashes(parents)) != wp.pu.View().ControlHash {
				log.Error().Str("where", "DecodeParents").Msg("wrong control hash")
				wp.failed = true
				return
			}
		default:
			log.Error().Str("where", "DecodeParents").Msg(err.Error())
			wp.failed = true
			return
		}
	}

	freeUnit := ad.dag.BuildUnit(wp.pu, parents)

	ad.alert.Lock(freeUnit.Creator())
	defer ad.alert.Unlock(freeUnit.Creator())

	err = ad.dag.Check(freeUnit)
	err = ad.alert.ResolveMissingCommitment(err, freeUnit, wp.source)
	if err != nil {
		log.Error().Str("where", "Check").Msg(err.Error())
		wp.failed = true
		return
	}
	unitInDag := ad.dag.Transform(freeUnit)
	ad.dag.Insert(unitInDag)
	log.Debug().Msg(logging.UnitAdded)
}

// checkCorrectness checks very basic correctness of the given preunit: creator and signature.
func (ad *adder) checkCorrectness(pu gomel.Preunit) error {
	if pu.Creator() >= ad.dag.NProc() {
		return gomel.NewDataError("invalid creator")
	}
	if ad.keys != nil && !ad.keys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("invalid signature")
	}
	return nil
}
