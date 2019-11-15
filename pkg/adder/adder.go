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
	decHandlers []gomel.DecodeErrorHandler
	chkHandlers []gomel.CheckErrorHandler
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
func New(dag gomel.Dag, keys []gomel.PublicKey, log zerolog.Logger) (gomel.Adder, gomel.Service) {
	ready := make([]chan *waitingPreunit, dag.NProc())
	for i := range ready {
		ready[i] = make(chan *waitingPreunit, 32)
	}
	ad := &adder{
		dag:         dag,
		keys:        keys,
		ready:       ready,
		waiting:     make(map[gomel.Hash]*waitingPreunit),
		waitingByID: make(map[uint64]*waitingPreunit),
		missing:     make(map[uint64]*missingPreunit),
		log:         log,
	}
	return ad, ad
}

func (ad *adder) AddDecodeErrorHandler(h gomel.DecodeErrorHandler) {
	ad.decHandlers = append(ad.decHandlers, h)
}

func (ad *adder) AddCheckErrorHandler(h gomel.CheckErrorHandler) {
	ad.chkHandlers = append(ad.chkHandlers, h)
}

// AddUnit checks basic correctness of a preunit and then adds it to the buffer zone.
// Does not block - this method returns when the preunit is added to the waiting preunits.
// The returned error can be:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents  - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddUnit(pu gomel.Preunit, source uint16) error {
	ad.log.Debug().Int(logging.Height, pu.Height()).Uint16(logging.Creator, pu.Creator()).Uint16(logging.PID, source).Msg(logging.AddUnitStarted)
	// SHALL BE DONE: this makes us vulnerable to spamming with the same units over and over.
	// We need a unit registry that remembers all the units that passed signature check
	err := ad.checkCorrectness(pu)
	if err != nil {
		return err
	}
	ad.mx.Lock()
	defer ad.mx.Unlock()
	if u := ad.dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
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
	// SHALL BE DONE: this makes us vulnerable to spamming with the same units over and over.
	// We need a unit registry that remembers all the units that passed signature check
	errors := make([]error, len(preunits))
	for i, pu := range preunits {
		err := ad.checkCorrectness(pu)
		if err != nil {
			errors[i] = err
			preunits[i] = nil
		}
	}
	hashes := make([]*gomel.Hash, len(preunits))
	for i, pu := range preunits {
		if pu != nil {
			hashes[i] = pu.Hash()
		}
	}
	ad.mx.Lock()
	defer ad.mx.Unlock()
	alreadyInDag := ad.dag.GetUnits(hashes)
	for i, pu := range preunits {
		if pu == nil {
			continue
		}
		if alreadyInDag[i] != nil {
			errors[i] = gomel.NewDuplicateUnit(alreadyInDag[i])
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
		log.Debug().Msg(logging.DecodeParentsError)
		for _, handler := range ad.decHandlers {
			if parents, err = handler(wp.pu, err, wp.source); err == nil {
				break
			}
		}
		if err != nil {
			log.Error().Str("where", "DecodeParents").Msg(err.Error())
			return
		}
	}
	freeUnit := ad.dag.BuildUnit(wp.pu, parents)
	err = ad.dag.Check(freeUnit)
	if err != nil {
		log.Debug().Msg(logging.CheckError)
		for _, handler := range ad.chkHandlers {
			if err = handler(freeUnit, err, wp.source); err == nil {
				break
			}
		}
		if err != nil {
			log.Error().Str("where", "Check").Msg(err.Error())
			return
		}
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
