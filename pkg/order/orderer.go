package order

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
)

const (
	beltSize = 1000
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
	orderedUnits chan []gomel.Unit
	mx           sync.RWMutex
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewOrderer TODO
func NewOrderer(conf config.Config, rsf gomel.RandomSourceFactory, ds core.DataSource, ps core.PreblockSink, log zerolog.Logger) gomel.Orderer {
	ord := &orderer{
		conf:         conf,
		rsf:          rsf,
		ps:           ps,
		unitBelt:     make(chan gomel.Unit, beltSize),
		orderedUnits: make(chan []gomel.Unit, 10),
		log:          log,
	}
	ord.creator = newCreator(conf, ord, ds, ord.unitBelt, log)
	return ord
}

func (ord *orderer) SetAlerter(alerter gomel.Alerter) {
	ord.alerter = alerter
}

func (ord *orderer) SetSyncer(syncer gomel.Syncer) {
	ord.syncer = syncer
}

func (ord *orderer) Start() error {
	ord.wg.Add(1)
	go ord.preblockMaker()
	return nil
}

func (ord *orderer) Stop() {
	close(ord.orderedUnits)
	ord.previous.close()
	ord.current.close()
	ord.wg.Wait()
}

func (ord *orderer) preblockMaker() {
	defer ord.wg.Done()
	current := gomel.EpochID(0)
	for round := range ord.orderedUnits {
		epoch := round[0].EpochID()
		if epoch >= current {
			ord.ps <- gomel.ToPreblock(round)
		}
		current = epoch
	}
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
			if witness(preunits[0]) {
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
// TODO this could potentially be counterproductive, as we gossip only about our most recent epoch.
//   That means just after switching to a new epoch due to "external proof", we immediately abandon the
//   previous epoch, even though we might still benefit from one last gossip to help us produce last timing units.
//   A potential solution would be to access the last-produced-preblock-epoch variable kept by preblockMaker() and
//   gossip also about "previous" if we haven't produced any preblock from "current".
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
	if epoch > ord.current.id {
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

func (ord *orderer) newEpoch(epoch gomel.EpochID) *epoch {
	ord.mx.Lock()
	defer ord.mx.Unlock()
	if epoch > ord.current.id {
		ord.previous.close()
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

func (ord *orderer) insert(unit gomel.Unit) {
	ep, newer := ord.getEpoch(unit.EpochID())
	if newer {
		ep = ord.newEpoch(unit.EpochID())
	}
	if ep != nil {
		ep.dag.Insert(unit)
	}
}

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
		return nil
	}
	return result
}

// witness checks if the given preunit is a proof that a new epoch started.
func witness(pu gomel.Preunit) bool {
	if !gomel.Dealing(pu) {
		return false
	}
	// TODO check threshold signature
	return true
}
