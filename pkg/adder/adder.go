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

// AddOwnUnit adds to the dag a unit produced by the same process. It blocks until unit is added, and returns it.
func (ad *adder) AddOwnUnit(pu gomel.Preunit) gomel.Unit {
	wp := &waitingPreunit{pu: pu, source: pu.Creator()}
	ad.handleReady(wp)
	return ad.dag.GetUnit(pu.Hash())
}

// AddUnit checks basic correctness of a preunit and then adds it to the buffer zone.
// Does not block - this method returns when the preunit is added to the waiting preunits.
// The returned error can be:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
//   UnknownParents  - in that case the preunit is normally added and processed, error is returned only for log purpose.
func (ad *adder) AddUnit(pu gomel.Preunit, source uint16) error {
	// SHALL BE DONE: unit registry check here
	err := ad.checkCorrectness(pu)
	if err != nil {
		return err
	}
	return ad.addOne(pu, source)
}

// AddUnit checks basic correctness of a given slice of preunits and then adds them to the buffer zone.
// It is assumed all preunits are sorted topologically and can be directly added to the dag (no missing parents).
// Returned AggregateError can have the following members:
//   DataError - if creator or signature are wrong
//   DuplicateUnit, DuplicatePreunit - if such a unit is already in dag/waiting
func (ad *adder) AddUnits(preunits []gomel.Preunit, source uint16) *gomel.AggregateError {
	// SHALL BE DONE: unit registry check here
	errors := make([]error, len(preunits))
	for i, pu := range preunits {
		err := ad.checkCorrectness(pu)
		if err != nil {
			errors[i] = err
			preunits[i] = nil
		}
	}
	return gomel.NewAggregateError(ad.addBatch(preunits, source, errors))
}

// Start the adding workers.
func (ad *adder) Start() error {
	ad.wg.Add(int(ad.dag.NProc()))
	for i := range ad.ready {
		go func(i int) {
			defer ad.wg.Done()
			for wp := range ad.ready[i] {
				ad.handleReady(wp)
				ad.remove(wp)
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

// sendToWorker takes a ready waitingPreuint and adds it to the channel corresponding
// with its dedicated worker. Atomic flag prevents send on a closed channel after Stop().
func (ad *adder) sendToWorker(wp *waitingPreunit) {
	if atomic.LoadInt64(&ad.quit) == 0 {
		ad.ready[wp.pu.Creator()] <- wp
	}
}

// handleReady takes a waitingPreunit that is ready and adds it to the dag.
func (ad *adder) handleReady(wp *waitingPreunit) {
	parents, err := ad.dag.DecodeParents(wp.pu)
	if err != nil {
		for _, handler := range ad.decHandlers {
			if parents, err = handler(wp.pu, err, wp.source); err == nil {
				break
			}
		}
		if err != nil {
			ad.log.Error().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Msg(err.Error())
			return
		}
	}
	freeUnit := ad.dag.BuildUnit(wp.pu, parents)
	err = ad.dag.Check(freeUnit)
	if err != nil {
		for _, handler := range ad.chkHandlers {
			if err = handler(freeUnit, err, wp.source); err == nil {
				break
			}
		}
		if err != nil {
			ad.log.Error().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Msg(err.Error())
			return
		}
	}
	unitInDag := ad.dag.Transform(freeUnit)
	ad.dag.Insert(unitInDag)
	ad.log.Info().Int(logging.Height, unitInDag.Height()).Uint16(logging.Creator, unitInDag.Creator()).Uint16(logging.PID, wp.source).Msg(logging.UnitAdded)
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
