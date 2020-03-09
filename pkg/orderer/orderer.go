// Package orderer contains the main implementation of the gomel.Orderer interface.
package orderer

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creator"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/core-go/pkg/core"
)

const (
	beltSize = 10000
)

type orderer struct {
	blockLimit   int
	conf         config.Config
	syncer       gomel.Syncer
	rsf          gomel.RandomSourceFactory
	alerter      gomel.Alerter
	toPreblock   gomel.PreblockMaker
	creator      *creator.Creator
	current      *epoch
	previous     *epoch
	unitBelt     chan gomel.Unit // Note: units on the unit belt does not have to appear in topological order
	lastTiming   chan gomel.Unit // used to pass the last timing unit of the epoch to creator
	orderedUnits chan []gomel.Unit
	mx           sync.RWMutex
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// New constructs a new orderer instance using provided config, data source, preblock maker, and logger.
func New(conf config.Config, ds core.DataSource, toPreblock gomel.PreblockMaker, log zerolog.Logger) gomel.Orderer {
	ord := &orderer{
		blockLimit:   conf.OrderStartLevel + conf.EpochLength,
		conf:         conf,
		toPreblock:   toPreblock,
		unitBelt:     make(chan gomel.Unit, beltSize),
		lastTiming:   make(chan gomel.Unit, 10),
		orderedUnits: make(chan []gomel.Unit, 10),
		log:          log.With().Int(logging.Service, logging.OrderService).Logger(),
	}
	send := func(u gomel.Unit) {
		ord.insert(u)
		ord.syncer.Multicast(u)
	}
	ord.creator = creator.New(conf, ds, send, ord.rsData, log)
	return ord
}

func (ord *orderer) Start(rsf gomel.RandomSourceFactory, syncer gomel.Syncer, alerter gomel.Alerter) {
	ord.rsf = rsf
	ord.syncer = syncer
	ord.alerter = alerter
	syncer.Start()
	alerter.Start()

	ord.wg.Add(1)
	go func() {
		defer ord.wg.Done()
		ord.creator.Work(ord.unitBelt, ord.lastTiming)
	}()

	ord.wg.Add(1)
	go func() {
		defer ord.wg.Done()
		ord.preblockMaker()
	}()

	ord.log.Info().Msg(logging.ServiceStarted)
}

func (ord *orderer) Stop() {
	ord.alerter.Stop()
	ord.syncer.Stop()
	if ord.previous != nil {
		ord.previous.close()
	}
	ord.current.close()
	close(ord.orderedUnits)
	close(ord.unitBelt)
	ord.wg.Wait()
	ord.log.Info().Msg(logging.ServiceStopped)
}

// preblockMaker waits for ordered round of units produced by Extenders and produces Preblocks based on them.
// Since Extenders in multiple epochs can supply ordered rounds simultaneously, preblockMaker needs to ensure that
// Preblocks are produced in ascending order with respect to epochs. For the last ordered round
// of the epoch, the timing unit defining it is sent to the creator (to produce signature shares.)
func (ord *orderer) preblockMaker() {
	current := gomel.EpochID(0)
	for round := range ord.orderedUnits {
		timingUnit := round[len(round)-1]
		if timingUnit.Level() == ord.blockLimit-1 {
			ord.lastTiming <- timingUnit
		}
		epoch := timingUnit.EpochID()
		if epoch >= current && timingUnit.Level() < ord.blockLimit {
			ord.toPreblock(round)
		}
		current = epoch
	}
	close(ord.lastTiming)
}

// AddPreunits sends preunits received from other committee members to their corresponding epochs.
// It assumes preunits are ordered by ascending epochID and, within each epoch, they are topologically sorted.
func (ord *orderer) AddPreunits(source uint16, preunits ...gomel.Preunit) []error {
	var errors []error
	errorsSize := len(preunits)
	getErrors := func() []error {
		if errors == nil {
			errors = make([]error, errorsSize)
		}
		return errors
	}
	processed := 0
	for len(preunits) > 0 {
		epoch := preunits[0].EpochID()
		end := 0
		for end < len(preunits) && preunits[end].EpochID() == epoch {
			end++
		}
		ep, newer := ord.getEpoch(epoch)
		if newer {
			if creator.EpochProof(preunits[0], ord.conf.WTKey) {
				ep = ord.newEpoch(epoch)
			} else {
				// TODO: don't do this if preunits[0] is too high
				ord.syncer.RequestGossip(source)
			}
		}
		if ep != nil {
			errs := ep.adder.AddPreunits(source, preunits[:end]...)
			copy(getErrors()[processed:], errs)
		}
		preunits = preunits[end:]
		processed += end
	}
	return errors
}

// UnitsByID allows to access units present in the orderer using their ids.
// The returned slice contains only existing units (no nil entries for non-present units)
// and can contain multiple units with the same id (forks). Because of that the length
// of the result can be different than the number of arguments.
func (ord *orderer) UnitsByID(ids ...uint64) []gomel.Unit {
	var result []gomel.Unit
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	for _, id := range ids {
		_, _, epoch := gomel.DecodeID(id)
		if epoch == ord.current.id {
			result = append(result, ord.current.dag.GetByID(id)...)
			continue
		}
		if epoch == ord.previous.id {
			result = append(result, ord.previous.dag.GetByID(id)...)
		}
	}
	return result
}

// UnitsByHash allows to access units present in the orderer using their hashes.
// The length of the returned slice is equal to the number of argument hashes.
// For non-present units the returned slice contains nil on the corresponding position.
func (ord *orderer) UnitsByHash(hashes ...*gomel.Hash) []gomel.Unit {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	result := ord.current.dag.GetUnits(hashes)
	for i := range result {
		if result[i] == nil {
			result[i] = ord.previous.dag.GetUnit(hashes[i])
		}
	}
	return result
}

// MaxUnits returns maximal units per process from the chosen epoch.
// TODO this is used only by Alerts, maybe a different signature would be more convenient?
func (ord *orderer) MaxUnits(epoch gomel.EpochID) gomel.SlottedUnits {
	ep, _ := ord.getEpoch(epoch)
	if ep != nil {
		return ep.dag.MaximalUnitsPerProcess()
	}
	return nil
}

// GetInfo returns DagInfo of the dag from the most recent epoch.
// TODO: don't always include previous info. Come up with heuristics for that.
func (ord *orderer) GetInfo() [2]*gomel.DagInfo {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	var result [2]*gomel.DagInfo
	if ord.previous != nil {
		result[0] = gomel.MaxView(ord.previous.dag)
	}
	if ord.current != nil {
		result[1] = gomel.MaxView(ord.current.dag)
	}
	return result
}

// Delta returns all units present in the orderer that are newer than units
// described by the given DagInfo. This includes all units from the epoch given
// by the DagInfo above provided heights as well as ALL units from newer epochs.
func (ord *orderer) Delta(info [2]*gomel.DagInfo) []gomel.Unit {
	ord.mx.RLock()
	defer ord.mx.RUnlock()

	var result []gomel.Unit
	deltaResolver := func(dagInfo *gomel.DagInfo) {
		if dagInfo == nil {
			return
		}
		if ord.previous != nil && dagInfo.Epoch == ord.previous.id {
			result = append(result, ord.previous.unitsAbove(dagInfo.Heights)...)
		}
		if ord.current != nil && dagInfo.Epoch == ord.current.id {
			result = append(result, ord.current.unitsAbove(dagInfo.Heights)...)
		}
	}
	deltaResolver(info[0])
	deltaResolver(info[1])
	if info[0] != nil && info[0].Epoch < ord.current.id && info[1].Epoch < ord.current.id {
		result = append(result, ord.current.allUnits()...)
	}
	return result
}

// getEpoch returns epoch with the given EpochID. If no such epoch is present,
// the second returned value indicates if the requested epoch is newer than current.
func (ord *orderer) getEpoch(epoch gomel.EpochID) (*epoch, bool) {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	if ord.current == nil || epoch > ord.current.id {
		return nil, true
	}
	if epoch == ord.current.id {
		return ord.current, false
	}
	if epoch == ord.previous.id {
		return ord.previous, false
	}
	return nil, false
}

// newEpoch creates and returns a new epoch object with the given EpochID. If such epoch already exists, returns it.
func (ord *orderer) newEpoch(epoch gomel.EpochID) *epoch {
	ord.mx.Lock()
	defer ord.mx.Unlock()
	if ord.current == nil || epoch > ord.current.id {
		if ord.previous != nil {
			ord.previous.close()
		}
		ord.previous = ord.current
		ord.current = newEpoch(epoch, ord.conf, ord.syncer, ord.rsf, ord.alerter, ord.unitBelt, ord.orderedUnits, ord.log)
		return ord.current
	}
	if epoch == ord.current.id {
		return ord.current
	}
	if epoch == ord.previous.id {
		return ord.previous
	}
	return nil
}

// insert puts the provided unit directly into the corresponding epoch. If such epoch does not exist, creates it.
// All correctness checks (epoch proof, adder, dag checks) are skipped. This method is meant for our own units only.
func (ord *orderer) insert(unit gomel.Unit) {
	if unit.Creator() == ord.conf.Pid {
		ep, newer := ord.getEpoch(unit.EpochID())
		if newer {
			ep = ord.newEpoch(unit.EpochID())
		}
		if ep != nil {
			ep.dag.Insert(unit)
			ord.log.Info().Int(logging.Height, unit.Height()).Msg(logging.UnitAdded)
		}
	}
}

// rsData produces random source data for a unit with provided level, parents and epoch.
func (ord *orderer) rsData(level int, parents []gomel.Unit, epoch gomel.EpochID) []byte {
	var result []byte
	var err error
	if level == 0 {
		result, err = ord.rsf.DealingData(epoch)
	} else {
		ep, _ := ord.getEpoch(epoch)
		if ep != nil {
			result, err = ep.rs.DataToInclude(parents, level)
		} else {
			err = gomel.NewDataError("unknown epoch")
		}
	}
	if err != nil {
		ord.log.Error().Str("where", "orderer.rsData").Msg(err.Error())
		return []byte{}
	}
	return result
}
