// Package creator contains the implementation of Creator together with functions for serializing and deserializing new epoch proofs.
package creator

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
)

// Creator is a component responsible for producing new units. It reads units produced by other
// committee members from some external channel (aka unit belt) and stores the ones with the highest
// level as possible parents (candidates). Whenever there are enough parents to produce a unit on a new level,
// Creator collects data from its DataSource, and random source data using the provided function, then builds,
// signs and sends (using a function given to the constructor) a new unit.
type Creator struct {
	conf       config.Config
	ds         core.DataSource
	send       func(gomel.Unit)
	rsData     func(int, []gomel.Unit, gomel.EpochID) []byte
	epoch      gomel.EpochID
	epochDone  bool
	candidates []gomel.Unit
	quorum     uint16
	maxLvl     int    // max level of units in candidates
	onMaxLvl   uint16 // number of candidates on maxLvl
	level      int    // level of unit we could produce with current candidates
	shares     *shareDB
	frozen     map[uint16]bool
	mx         sync.Mutex
	finished   bool
	log        zerolog.Logger
}

// New constructs a creator that uses provided config, data source and logger.
// send function is called on each created unit.
// rsData provides random source data for the given level, parents and epoch.
func New(conf config.Config, dataSource core.DataSource, send func(gomel.Unit), rsData func(int, []gomel.Unit, gomel.EpochID) []byte, log zerolog.Logger) *Creator {
	return &Creator{
		conf:       conf,
		ds:         dataSource,
		send:       send,
		rsData:     rsData,
		candidates: make([]gomel.Unit, conf.NProc),
		maxLvl:     -1,
		quorum:     gomel.MinimalQuorum(conf.NProc),
		shares:     newShareDB(conf),
		frozen:     make(map[uint16]bool),
		log:        log,
	}
}

// Work executes the main loop of the creator. Units appearing on unitBelt are examined and stored to
// be used as parents of future units. When there are enough new parents, a new unit is produced.
// lastTiming is a channel on which the last timing unit of each epoch is expected to appear.
// This method is stopped by closing unitBelt channel.
func (cr *Creator) Work(unitBelt, lastTiming <-chan gomel.Unit, alerter gomel.Alerter) {
	defer func() {
		cr.log.Info().Msg(logging.CreatorFinished)
	}()
	om := alerter.AddForkObserver(func(u, _ gomel.Preunit) {
		cr.freezeParent(u.Creator())
	})
	defer om.RemoveObserver()
	cr.newEpoch(gomel.EpochID(0), core.Data{})

	var parents []gomel.Unit
	var level int

	for u := range unitBelt {
		if cr.finished {
			return
		}
		cr.mx.Lock()
		cr.update(u)
		if cr.ready() {
			// Step 1: update candidates with all units waiting on the unit belt
			n := len(unitBelt)
			for i := 0; i < n; i++ {
				cr.update(<-unitBelt)
			}
			if cr.ready() {
				// we need to check that again, in case epoch changed in Step 1.
				// Step 2: pick parents and level depending on creating strategy
				if cr.conf.CanSkipLevel {
					level = cr.level
					parents = cr.getParents()
				} else {
					level = cr.candidates[cr.conf.Pid].Level() + 1
					parents = cr.getParentsForLevel(level)
				}
				// Step 3: create unit
				cr.createUnit(parents, level, cr.getData(level, lastTiming))
			}
		}
		cr.mx.Unlock()
	}
}

// ready checks if the creator is ready to produce a new unit. Usually that means:
// "do we have enough new candidates to produce a unit with level higher than the previous one?"
// Besides that, we stop producing units for the current epoch after creating a unit with signature share.
func (cr *Creator) ready() bool {
	return !cr.epochDone && cr.level > cr.candidates[cr.conf.Pid].Level()
}

// getData produces a piece of data to be included in a unit on a given level.
// For regular units the provided DataSource is used
// For finishing units it's either nil or, if available, an encoded threshold signature share
// of hash and id of the last timing unit (obtained from preblockMaker on lastTiming channel)
func (cr *Creator) getData(level int, lastTiming <-chan gomel.Unit) core.Data {
	if level < cr.conf.OrderStartLevel+cr.conf.EpochLength {
		if cr.ds != nil {
			return cr.ds.GetData()
		}
		return core.Data{}
	}
	for {
		// in a rare case there can be timing units from previous epochs left on lastTiming channel.
		// the purpose of this loop is to drain and ignore them.
		select {
		case timingUnit := <-lastTiming:
			if timingUnit.EpochID() < cr.epoch {
				continue
			}
			if timingUnit.EpochID() == cr.epoch {
				cr.epochDone = true
				if int(cr.epoch) == cr.conf.NumberOfEpochs-1 {
					// the epoch we just finished is the last epoch we were supposed to produce
					return core.Data{}
				}
				msg := encodeProof(timingUnit)
				share := cr.conf.WTKey.CreateShare(msg)
				if share != nil {
					return encodeShare(share, msg)
				}
				return core.Data{}
			}
			panic("TIME TRAVEL ERROR: lastTiming received a unit from the future")
		default:
			return core.Data{}
		}
	}
}

// update takes a unit that has been received from unit belt and updates
// creator internal state with information contained in that unit.
func (cr *Creator) update(u gomel.Unit) {
	cr.log.Debug().
		Uint16(logging.Creator, u.Creator()).
		Uint32(logging.Epoch, uint32(u.EpochID())).
		Int(logging.Height, u.Height()).
		Int(logging.Level, u.Level()).
		Msg(logging.CreatorProcessingUnit)

	// if the unit is from an older epoch or unit's creator is known to be a forker, we simply ignore it
	if cr.frozen[u.Creator()] || u.EpochID() < cr.epoch {
		return
	}

	// If the unit is from a new epoch, switch to that epoch.
	// Since units appear on the belt in order they were added to the dag,
	// the first unit from a new epoch is always a dealing unit.
	if u.EpochID() > cr.epoch {
		if !EpochProof(u, cr.conf.WTKey) {
			cr.log.Warn().
				Uint16(logging.Creator, u.Creator()).
				Int(logging.Height, u.Height()).
				Msg(logging.InvalidEpochProofFromFuture)
			return
		}
		cr.newEpoch(u.EpochID(), u.Data())
	}

	// If this is a finishing unit try to extract threshold signature share from it.
	// If there are enough shares to produce the signature (and therefore a proof that
	// the current epoch is finished) switch to a new epoch.
	data := cr.updateShares(u)
	if data != nil {
		cr.newEpoch(cr.epoch+1, data)
		return
	}

	cr.updateCandidates(u)
}

// updateCandidates puts the provided unit in parent candidates provided that
// the level is higher than the level of the previous candidate for that creator.
func (cr *Creator) updateCandidates(u gomel.Unit) {
	if u.EpochID() != cr.epoch {
		return
	}
	prev := cr.candidates[u.Creator()]
	if prev == nil || prev.Level() < u.Level() {
		cr.candidates[u.Creator()] = u
		if u.Level() == cr.maxLvl {
			cr.onMaxLvl++
		}
		if u.Level() > cr.maxLvl {
			cr.maxLvl = u.Level()
			cr.onMaxLvl = 1
		}
		cr.level = cr.maxLvl
		if cr.onMaxLvl >= cr.quorum {
			cr.level++
		}
	}
}

// resetCandidates resets the candidates and all related variables to the initial state
// (a slice with NProc nils). This is useful when switching to a new epoch.
func (cr *Creator) resetCandidates() {
	for ix := range cr.candidates {
		cr.candidates[ix] = nil
	}
	cr.maxLvl = -1
	cr.onMaxLvl = 0
	cr.level = 0
}

// updateShares extracts threshold signature shares from finishing units.
// If there are enough shares to combine, it produces the signature and
// converts it to core.Data. Otherwise, nil is returned.
func (cr *Creator) updateShares(u gomel.Unit) core.Data {
	// ignore regular units and finishing units with empty data
	if u.Level() < cr.conf.OrderStartLevel+cr.conf.EpochLength || len(u.Data()) == 0 {
		return nil
	}
	share, msg, err := decodeShare(u.Data())
	if err != nil {
		cr.log.Error().Str("where", "creator.decodeShare").Msg(err.Error())
		return nil
	}
	if !cr.conf.WTKey.VerifyShare(share, msg) {
		cr.log.Error().Str("where", "creator.verifyShare").Msg(err.Error())
		return nil
	}
	sig := cr.shares.Add(share, msg)
	if sig != nil {
		return encodeSignature(sig, msg)
	}
	return nil
}

// freezeParent tells the creator to stop updating parent candidates for the given pid
// and use the corresponding parent of our last created unit instead. Returns that parent.
func (cr *Creator) freezeParent(pid uint16) gomel.Unit {
	cr.mx.Lock()
	defer cr.mx.Unlock()
	u := cr.candidates[cr.conf.Pid].Parents()[pid]
	cr.candidates[pid] = u
	cr.frozen[pid] = true
	cr.log.Info().Uint16(logging.Creator, pid).Msg(logging.FreezedParent)
	return u
}

// getParents returns a copy of current parent candidates.
func (cr *Creator) getParents() []gomel.Unit {
	result := make([]gomel.Unit, cr.conf.NProc)
	copy(result, cr.candidates)
	MakeConsistent(result)
	return result
}

// getParentsForLevel returns a set of candidates such that their level is at most level-1.
func (cr *Creator) getParentsForLevel(level int) []gomel.Unit {
	result := make([]gomel.Unit, cr.conf.NProc)
	for i, u := range cr.candidates {
		for u != nil && u.Level() >= level {
			u = gomel.Predecessor(u)
		}
		result[i] = u
	}
	MakeConsistent(result)
	return result
}

// createUnit creates a unit with the given parents, level, and data. Assumes provided parameters
// are consistent, that means level == gomel.LevelFromParents(parents) and cr.epoch == parents[i].EpochID()
func (cr *Creator) createUnit(parents []gomel.Unit, level int, data core.Data) {
	rsData := cr.rsData(level, parents, cr.epoch)
	u := unit.New(cr.conf.Pid, cr.epoch, parents, level, data, rsData, cr.conf.PrivateKey)
	cr.log.Info().
		Uint32(logging.Epoch, uint32(u.EpochID())).
		Int(logging.Height, u.Height()).
		Int(logging.Level, level).
		Msg(logging.UnitCreated)
	cr.send(u)
	cr.update(u)
}

// newEpoch switches the creator to a chosen epoch, resets candidates and shares and creates a dealing with the provided data.
func (cr *Creator) newEpoch(epoch gomel.EpochID, data core.Data) {
	cr.epoch = epoch
	cr.epochDone = false
	cr.resetCandidates()
	cr.shares.Reset()
	if epoch >= gomel.EpochID(cr.conf.NumberOfEpochs) {
		cr.finished = true
		return
	}
	cr.log.Info().Uint32(logging.Epoch, uint32(epoch)).Msg(logging.CreatorSwitchedToNewEpoch)
	cr.createUnit(make([]gomel.Unit, cr.conf.NProc), 0, data)
}

// MakeConsistent ensures that the set of parents follows "parent consistency rule". Modifies the provided unit slice in place.
// Parent consistency rule means that unit's i-th parent cannot be lower (in a level sense) than
// i-th parent of any other of that units parents. In other words, units seen from U "directly"
// (as parents) cannot be below the ones seen "indirectly" (as parents of parents).
func MakeConsistent(parents []gomel.Unit) {
	for i := 0; i < len(parents); i++ {
		for j := 0; j < len(parents); j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil || (u != nil && u.Level() > parents[i].Level()) {
				parents[i] = u
			}
		}
	}
}
