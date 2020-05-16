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
	conf         config.Config
	syncer       gomel.Syncer
	rsf          gomel.RandomSourceFactory
	alerter      gomel.Alerter
	toPreblock   gomel.PreblockMaker
	ds           core.DataSource
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
	return &orderer{
		conf:         conf,
		toPreblock:   toPreblock,
		ds:           ds,
		unitBelt:     make(chan gomel.Unit, beltSize),
		lastTiming:   make(chan gomel.Unit, 10),
		orderedUnits: make(chan []gomel.Unit, 10),
		log:          log.With().Int(logging.Service, logging.OrderService).Logger(),
	}
}

func (ord *orderer) Start(rsf gomel.RandomSourceFactory, syncer gomel.Syncer, alerter gomel.Alerter) {
	ord.rsf = rsf
	ord.syncer = syncer
	ord.alerter = alerter

	send := func(u gomel.Unit) {
		ord.insert(u)
		ord.syncer.Multicast(u)
	}
	epochProofBuilder := creator.NewProofBuilder(ord.conf, ord.log)
	ord.creator = creator.New(ord.conf, ord.ds, send, ord.rsData, epochProofBuilder, ord.log)

	ord.newEpoch(gomel.EpochID(0))

	syncer.Start()
	alerter.Start()

	ord.wg.Add(1)
	go func() {
		defer ord.wg.Done()
		ord.creator.CreateUnits(ord.unitBelt, ord.lastTiming, alerter)
	}()

	ord.wg.Add(1)
	go func() {
		defer ord.wg.Done()
		ord.handleTimingRounds()
	}()

	ord.log.Log().Msg(logging.ServiceStarted)
}

func (ord *orderer) Stop() {
	ord.alerter.Stop()
	ord.syncer.Stop()
	if ord.previous != nil {
		ord.previous.Close()
	}
	if ord.current != nil {
		ord.current.Close()
	}
	close(ord.orderedUnits)
	close(ord.unitBelt)
	ord.wg.Wait()
	ord.log.Log().Msg(logging.ServiceStopped)
}

// handleTimingRounds waits for ordered round of units produced by Extenders and produces Preblocks based on them.
// Since Extenders in multiple epochs can supply ordered rounds simultaneously, handleTimingRounds needs to ensure that
// Preblocks are produced in ascending order with respect to epochs. For the last ordered round
// of the epoch, the timing unit defining it is sent to the creator (to produce signature shares.)
func (ord *orderer) handleTimingRounds() {
	defer close(ord.lastTiming)
	current := gomel.EpochID(0)
	for round := range ord.orderedUnits {
		timingUnit := round[len(round)-1]
		if timingUnit.Level() == ord.conf.LastLevel {
			ord.lastTiming <- timingUnit
			ord.finishEpoch(timingUnit.EpochID())
		}
		epoch := timingUnit.EpochID()
		if epoch >= current && timingUnit.Level() <= ord.conf.LastLevel {
			ord.toPreblock(round)
		}
		current = epoch
	}
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
		ep := ord.retrieveEpoch(preunits[0], source)
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
	result := make([]gomel.Unit, 0, len(ids))
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	for _, id := range ids {
		_, _, epoch := gomel.DecodeID(id)
		ep, _ := ord.getEpoch(epoch)
		if ep != nil {
			result = append(result, ep.dag.GetByID(id)...)
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
	var result []gomel.Unit
	if ord.current != nil {
		result = ord.current.dag.GetUnits(hashes)
	} else {
		result = make([]gomel.Unit, len(hashes))
	}
	if ord.previous != nil {
		for i := range result {
			if result[i] == nil {
				result[i] = ord.previous.dag.GetUnit(hashes[i])
			}
		}
	}
	return result
}

// MaxUnits returns maximal units per process from the chosen epoch.
func (ord *orderer) MaxUnits(epoch gomel.EpochID) gomel.SlottedUnits {
	ep, _ := ord.getEpoch(epoch)
	if ep != nil {
		return ep.dag.MaximalUnitsPerProcess()
	}
	return nil
}

// GetInfo returns DagInfo of the dag from the most recent epoch.
func (ord *orderer) GetInfo() [2]*gomel.DagInfo {
	ord.mx.RLock()
	defer ord.mx.RUnlock()
	var result [2]*gomel.DagInfo
	if ord.previous != nil && !ord.previous.IsFinished() {
		result[0] = gomel.MaxView(ord.previous.dag)
	}
	if ord.current != nil && !ord.current.IsFinished() {
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
	if ord.current != nil {
		if info[0] != nil && info[0].Epoch < ord.current.id && info[1] != nil && info[1].Epoch < ord.current.id {
			result = append(result, ord.current.allUnits()...)
		}
	}
	return result
}

func (ord *orderer) retrieveEpoch(pu gomel.Preunit, source uint16) *epoch {
	epochID := pu.EpochID()
	epoch, fromFuture := ord.getEpoch(epochID)
	if fromFuture {
		if creator.EpochProof(pu, ord.conf.WTKey) {
			epoch = ord.newEpoch(epochID)
		} else {
			ord.syncer.RequestGossip(source)
		}
	}
	return epoch
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
			ord.previous.Close()
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

func (ord *orderer) finishEpoch(epoch gomel.EpochID) {
	ep, _ := ord.getEpoch(epoch)
	if ep != nil {
		ep.Finish()
	}
}

// insert puts the provided unit directly into the corresponding epoch. If such epoch does not exist, creates it.
// All correctness checks (epoch proof, adder, dag checks) are skipped. This method is meant for our own units only.
func (ord *orderer) insert(unit gomel.Unit) {
	if unit.Creator() != ord.conf.Pid {
		ord.log.Warn().Uint16(logging.Creator, unit.Creator()).Msg(logging.InvalidCreator)
		return
	}
	ep, newer := ord.getEpoch(unit.EpochID())
	if newer {
		ep = ord.newEpoch(unit.EpochID())
	}
	if ep != nil {
		ep.dag.Insert(unit)
		ord.log.Info().
			Uint16(logging.Creator, unit.Creator()).
			Uint32(logging.Epoch, uint32(unit.EpochID())).
			Int(logging.Height, unit.Height()).
			Int(logging.Level, unit.Level()).
			Msg(logging.UnitAdded)
	} else {
		ord.log.Info().
			Uint32(logging.Epoch, uint32(unit.EpochID())).
			Int(logging.Height, unit.Height()).
			Int(logging.Level, unit.Level()).
			Msg(logging.UnableToRetrieveEpoch)
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
