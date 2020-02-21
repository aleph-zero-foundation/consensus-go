package epochs

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
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
	ps           core.PreblockSink
	current      *epoch
	previous     *epoch
	creator      *creator
	unitBelt     chan gomel.Unit
	lastTiming   chan gomel.Unit // used to pass the last timing unit of the epoch to creator
	orderedUnits chan []gomel.Unit
	mx           sync.RWMutex
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewOrderer TODO
//
// Note: units on the unit belt does not have to appear in topological order,
// but for a given creator they are ordered by ascending height.
func NewOrderer(conf config.Config, rsf gomel.RandomSourceFactory, ds core.DataSource, ps core.PreblockSink, log zerolog.Logger) gomel.Orderer {
	ord := &orderer{
		conf:         conf,
		rsf:          rsf,
		ps:           ps,
		unitBelt:     make(chan gomel.Unit, beltSize),
		lastTiming:   make(chan gomel.Unit, 10),
		orderedUnits: make(chan []gomel.Unit, 10),
		log:          log,
	}
	ord.creator = newCreator(conf, ord, ds, log)
	return ord
}

func (ord *orderer) SetAlerter(alerter gomel.Alerter) {
	ord.alerter = alerter
}

func (ord *orderer) SetSyncer(syncer gomel.Syncer) {
	ord.syncer = syncer
}

func (ord *orderer) Start() error {
	ord.creator.newEpoch(gomel.EpochID(0), core.Data{})
	go ord.creator.work()
	go ord.preblockMaker()
	return nil
}

func (ord *orderer) Stop() {
	ord.previous.close()
	ord.current.close()
	close(ord.orderedUnits)
	close(ord.unitBelt)
	ord.wg.Wait()
}

// preblockMaker waits for ordered round of units produced by Extenders and produces Preblocks based on them.
// Since Extenders in multiple epochs can supply ordered rounds simultaneously, preblockMaker needs to ensure that
// Preblocks are produced in ascending order with respect to epochs. For the last ordered round
// of the epoch, the timing unit defining it is sent to the creator (to produce signature shares.)
func (ord *orderer) preblockMaker() {
	ord.wg.Add(1)
	defer ord.wg.Done()
	current := gomel.EpochID(0)
	for round := range ord.orderedUnits {
		timingUnit := round[len(round)-1]
		if timingUnit.Level() == ord.conf.OrderStartLevel+ord.conf.EpochLength-1 {
			ord.lastTiming <- timingUnit
		}
		epoch := timingUnit.EpochID()
		if epoch >= current {
			ord.ps <- gomel.ToPreblock(round)
		}
		current = epoch
	}
	close(ord.lastTiming)
}

// AddPreunits sends preunits received from other committee members to their corresponding epochs.
// It assumes preunits are ordered by ascending epochID and, within each epoch, they are topologically sorted.
func (ord *orderer) AddPreunits(source uint16, preunits ...gomel.Preunit) {
	for len(preunits) > 0 {
		epoch := preunits[0].EpochID()
		end := 0
		for end < len(preunits) && preunits[end].EpochID() == epoch {
			end++
		}
		ep, newer := ord.getEpoch(epoch)
		if newer {
			if witness(preunits[0], conf.ThresholdKey) {
				ep = ord.newEpoch(epoch)
			} else {
				// TODO: don't do this if preunits[0] is too high
				ord.syncer.RequestGossip(source)
			}
		}
		if ep != nil {
			ep.adder.AddPreunits(source, preunits[:end]...) //TODO handle error
		}
		preunits = preunits[end:]
	}
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
		if ord.previous != nil && dagInfo.Epoch == ord.previous.id {
			result = append(result, ord.previous.unitsAbove(dagInfo.Heights)...)
		}
		if ord.current != nil && dagInfo.Epoch == ord.current.id {
			result = append(result, ord.current.unitsAbove(dagInfo.Heights)...)
		}
	}
	deltaResolver(info[0])
	deltaResolver(info[1])
	if info[0].Epoch < ord.current.id && info[1].Epoch < ord.current.id {
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
// All correctness checks (witness, adder, dag checks) are skipped. This method is meant for our own units only.
func (ord *orderer) insert(unit gomel.Unit) {
	if unit.Creator() == ord.conf.Pid {
		ep, newer := ord.getEpoch(unit.EpochID())
		if newer {
			ep = ord.newEpoch(unit.EpochID())
		}
		if ep != nil {
			ep.dag.Insert(unit)
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
