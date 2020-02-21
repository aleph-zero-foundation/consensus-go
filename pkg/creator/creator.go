package creator

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
)

// Creator is a component responsible for producing new units. TODO
type Creator struct {
	conf       config.Config
	ds         core.DataSource
	send       func(gomel.Unit)
	rsData     func(int, []gomel.Unit, gomel.EpochID) []byte
	epoch      gomel.EpochID
	epochDone  bool
	candidates []gomel.Unit
	maxLvl     int // max level of units in candidates
	onMaxLvl   int // number of candidates on maxLvl
	level      int // level of unit we could produce with current candidates
	quorum     int
	shares     map[string]bool
	frozen     map[uint16]bool
	mx         sync.Mutex
	log        zerolog.Logger
}

// New constructs a creator that uses provided config, data source and logger.
// send function is called on each created unit.
// rsData provides random source data for the given level, parents and epoch.
func New(conf config.Config, dataSource core.DataSource, send func(gomel.Unit), rsData func(int, []gomel.Unit, gomel.EpochID) []byte, log zerolog.Logger) *Creator {
	return &creator{
		conf:       conf,
		ds:         dataSource,
		candidates: make([]gomel.Unit, conf.NProc),
		maxLvl:     -1,
		quorum:     int(gomel.MinimalQuorum(conf.NProc)),
		shares:     newShareDB(),
		frozen:     make(map[uint16]bool),
		log:        log,
	}
}

// work executes the main loop of the creator. Units appearing on the unit belt are examined and stored to
// be used as parents of future units. If the creator is ready, a new unit is produced.
// This method is stopped by closing the unit belt.
func (cr *Creator) work(unitBelt, lastTiming <-chan gomel.Unit, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	var parents []gomel.Unit
	var level int

	for u := range unitBelt {
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
		return cr.ds.GetData()
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
				msg := encodeProof(timingUnit)
				share := cr.conf.ThresholdKey.CreateShare(msg)
				if share != nil {
					return encodeShare(share, msg)
				}
				return core.Data{}
			}
			panic("TIME TRAVEL ERROR: lastTiming received a unit from the future")
		default:
			break
		}
	}
	return core.Data{}
}

// update takes a unit that has been received from unit belt and updates
// creator internal state with information contained in that unit.
func (cr *Creator) update(u gomel.Unit) {
	// if the unit is from an older epoch or unit's creator is known to be a forker, we simply ignore it
	if cr.frozen[u.Creator()] || u.EpochID() < cr.epoch {
		return
	}

	// If the unit is from a new epoch, switch to that epoch.
	// Since units appear on the belt in order they were added to the dag,
	// the first unit from a new epoch is always a witness unit.
	if u.EpochID() > cr.epoch {
		if !witness(u, cr.conf.ThresholdKey) {
			panic("creator received non-witness unit from new epoch")
		}
		cr.newEpoch(u.EpochID(), u.Data())
		return
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
	cr.candidates = make([]gomel.Unit, cr.conf.NProc)
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
	if !cr.conf.ThresholdKey.Verify(share, msg) {
		cr.log.Error().Str("where", "creator.verifyShare").Msg(err.Error())
		return nil
	}
	sig := cr.shares.add(share, msg)
	if sig != nil {
		return marshallSignature(sig, msg)
	}
	return nil
}

// freezeParent tells the creator to stop updating parent candidates for the given pid
// and use the corresponding parent of our last created unit instead. Returns that parent.
// TODO: this should be called when a fork is discovered and we need to produce commitment.
func (cr *Creator) freezeParent(pid uint16) gomel.Unit {
	cr.mx.Lock()
	defer cr.mx.Unlock()
	u := cr.candidates[cr.conf.Pid].Parents()[pid]
	cr.candidates[pid] = u
	cr.frozen[pid] = true
	return u
}

// getParents returns a copy of current parent candidates.
func (cr *Creator) getParents() []gomel.Unit {
	result := make([]gomel.Unit, cr.conf.NProc)
	copy(result, cr.candidates)
	makeConsistent(result)
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
	makeConsistent(result)
	return result
}

// createUnit creates a unit with the given parents, level, and data. Assumes provided parameters
// are consistent, that means level == gomel.LevelFromParents(parents) and cr.epoch == parents[i].EpochID()
// Inserts the new unit into orderer and updates local info.
func (cr *Creator) createUnit(parents []gomel.Unit, level int, data core.Data) {
	rsData := cr.rsData(level, parents, cr.epoch)
	u := unit.New(cr.conf.Pid, cr.epoch, parents, level, data, rsData, cr.conf.PrivateKey)
	cr.send(u)
	cr.update(u)
}

// newEpoch switches the creator to a chosen epoch, resets candidates and shares and creates a dealing with the provided data.
func (cr *Creator) newEpoch(epoch gomel.EpochID, data core.Data) {
	cr.epoch = epoch
	cr.epochDone = false
	cr.resetCandidates()
	cr.shares.reset()
	cr.createUnit(make([]gomel.Unit, cr.conf.NProc), 0, data)
}

// makeConsistent ensures that the set of parents follows "parent consistency rule". Modifies the provided unit slice in place.
// Parent consistency rule means that unit's i-th parent cannot be lower (in a level sense) than
// i-th parent of any other of that units parents. In other words, units seen from U "directly"
// (as parents) cannot be below the ones seen "indirectly" (as parents of parents).
func makeConsistent(parents []gomel.Unit) {
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
