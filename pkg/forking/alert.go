// Package forking provides services and DAG wrappers that handle alerts raised when forks are encountered.
//
// When a fork is encountered a forking proof is constructed. The proof consists of two units created by the same process at the same height,
// and another unit created by the same process, which is the maximal unit created by this process that we can use as a direct parent.
// This last unit might be nil. This proof is then reliably multicast to all other processes.
//
// When we want to add a unit created by a known forker to our dag, we need a proof that it is below a unit that someone used as
// the third unit in their alert. For this purpose we use commitments. Commitments either refer to a raised alert directly by id,
// or prove that a unit is a predecessor of a unit to which we have a commitment.
package forking

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/rmc"
)

const (
	alert byte = iota
	proving
	finished
	request
)

// noCommitment is an error due to problems with missing commitments.
type noCommitment struct {
	msg string
}

func (e *noCommitment) Error() string {
	return "MissingCommitment: " + e.msg
}

func missingCommitment(msg string) *noCommitment {
	return &noCommitment{msg}
}

// alertHandler allows to raise alerts and handle commitments to units.
type alertHandler struct {
	myPid       uint16
	nProc       uint16
	dag         gomel.Dag
	keys        []gomel.PublicKey
	rmc         *rmc.RMC
	netserv     network.Server
	timeout     time.Duration
	commitments *commitBase
	locks       []sync.Mutex
	log         zerolog.Logger
}

// NewAlertHandler for raising and handling commitments.
func NewAlertHandler(myPid uint16, dag gomel.Dag, keys []gomel.PublicKey, rmc *rmc.RMC, netserv network.Server, timeout time.Duration, log zerolog.Logger) gomel.Alerter {
	nProc := uint16(len(keys))
	al := &alertHandler{
		myPid:       myPid,
		nProc:       nProc,
		dag:         dag,
		keys:        keys,
		rmc:         rmc,
		netserv:     netserv,
		timeout:     timeout,
		commitments: newCommitBase(),
		locks:       make([]sync.Mutex, nProc),
		log:         log,
	}
	dag.AddCheck(al.checkCommitment)
	return al
}

// HandleIncoming connection, either accepting an alert or responding to a commitment request.
func (a *alertHandler) HandleIncoming(conn network.Connection, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()
	pid, id, msgType, err := rmc.AcceptGreeting(conn)
	if err != nil {
		a.log.Error().Str("where", "alertHandler.handleIncoming.AcceptGreeting").Msg(err.Error())
		return
	}
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.ISID, id).Logger()
	log.Info().Msg(logging.SyncStarted)

	switch msgType {
	case alert:
		a.acceptAlert(id, pid, conn, log)
	case proving:
		a.acceptProof(id, conn, log)
	case finished:
		a.acceptFinished(id, pid, conn, log)
	case request:
		a.handleCommitmentRequest(conn, log)
	}
}

// acceptFinished alert. If this is the first alert pertaining this forker we are aware of, this method also raises our own alert.
func (a *alertHandler) acceptFinished(id uint64, pid uint16, conn network.Connection, log zerolog.Logger) {
	forker, _, err := a.decodeAlertID(id, pid)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptFinished.decodeAlertID").Msg(err.Error())
		return
	}
	data, err := a.rmc.AcceptFinished(id, pid, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptFinished.AcceptData").Msg(err.Error())
		return
	}
	proof, err := (&forkingProof{}).unmarshal(data)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptFinished.Unmarshal").Msg(err.Error())
		return
	}
	comm := proof.extractCommitment(id)
	a.commitments.add(comm, pid, forker)
	a.Lock(forker)
	defer a.Unlock(forker)
	if a.commitments.getByParties(a.myPid, pid) == nil {
		maxes := a.dag.MaximalUnitsPerProcess().Get(forker)
		if len(maxes) == 0 {
			proof.replaceCommit(nil)
		} else {
			proof.replaceCommit(maxes[0])
		}
		a.raiseAlert(proof)
	}
}

func (a *alertHandler) sendFinished(forker, pid uint16) {
	comm := a.commitments.getByParties(a.myPid, forker)
	if comm == nil {
		a.log.Error().Str("where", "alertHandler.sendFinished.getByParties").Msg("attempting to prove forking before alert finished")
		return
	}
	id := comm.rmcID()
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, id).Logger()
	conn, err := a.netserv.Dial(pid, a.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(a.timeout)
	log.Info().Msg(logging.SyncStarted)
	err = rmc.Greet(conn, a.myPid, id, finished)
	if err != nil {
		log.Error().Str("where", "alertHandler.sendFinished.Greet").Msg(err.Error())
		return
	}
	err = a.rmc.SendFinished(id, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.sendFinished.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "alertHandler.sendFinished.Flush").Msg(err.Error())
	}
}

// produceCommitmentFor the given unit. If we have already committed to a unit created by the same process as unit
// that is above unit, this method uses that commitment to infer a commitment fot unit.
// Otherwise returns an error.
func (a *alertHandler) produceCommitmentFor(unit gomel.Unit) (commitment, error) {
	comm := a.commitments.getByParties(a.myPid, unit.Creator())
	if comm == nil {
		return nil, errors.New("we are not aware of any forks here")
	}
	pu := comm.getUnit()
	if pu == nil {
		return nil, errors.New("we did not commit to anything")
	}
	commUnit := a.dag.GetUnit(pu.Hash())
	if commUnit == nil {
		return nil, errors.New("we do not have the unit we committed to")
	}
	pred := gomel.Predecessor(commUnit)
	for pred != nil && a.hasCommitmentTo(pred) {
		commUnit = pred
		pred = gomel.Predecessor(commUnit)
	}
	if pred == nil || commUnit.Height() <= unit.Height() {
		// Apparently we added the commitment in the meantime.
		comm = a.commitments.getByHash(unit.Hash())
		if comm == nil {
			return nil, errors.New("we are actually not committed to this unit")
		}
	} else {
		var err error
		comm = a.commitments.getByHash(commUnit.Hash())
		for commUnit.Height() > unit.Height() {
			comm, err = commitmentForParent(comm, commUnit)
			if err != nil {
				return nil, err
			}
			a.commitments.add(comm, a.myPid, commUnit.Creator())
			commUnit = gomel.Predecessor(commUnit)
		}
		if cu := comm.getUnit(); cu == nil || *cu.Hash() != *unit.Hash() {
			return nil, errors.New("produced commitment for wrong unit")
		}
	}
	return comm, nil
}

// handleCommitmentRequest coming from conn. If we are not aware that the creator is a forker, we respond with a 1 byte and end communication.
// Otherwise we respond with a 0 byte and send a commitment to the parent of the requested unit.
// This makes sure that the requesting party will be able to add the unit, even if its parents were ambiguous,
// as our commitment to the parent will allow them to cheaply decode the crown.
func (a *alertHandler) handleCommitmentRequest(conn network.Connection, log zerolog.Logger) {
	var requested gomel.Hash
	_, err := io.ReadFull(conn, requested[:])
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.ReadFull").Msg(err.Error())
		return
	}
	unit := a.dag.GetUnit(&requested)
	if unit == nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.Get").Msg("no commitment for unit not in dag")
		return
	}
	// We always want to send one commitment more if we can, so that we send the parents' hashes to add unit.
	if pred := gomel.Predecessor(unit); pred != nil {
		unit = pred
	}
	comm := a.commitments.getByHash(&requested)
	if comm == nil {
		if !a.IsForker(unit.Creator()) {
			log.Error().Str("where", "alertHandler.handleCommitmentRequest.getByHash").Msg("we were not aware there was a fork")
			_, err = conn.Write([]byte{1})
			if err != nil {
				log.Error().Str("where", "alertHandler.handleCommitmentRequest.Write").Msg(err.Error())
				return
			}
			err = conn.Flush()
			if err != nil {
				log.Error().Str("where", "alertHandler.handleCommitmentRequest.Flush").Msg(err.Error())
			}
			return
		}
		comm, err = a.produceCommitmentFor(unit)
		if err != nil {
			log.Error().Str("where", "alertHandler.handleCommitmentRequest.produceCommitmentFor").Msg(err.Error())
			return
		}
	}
	_, err = conn.Write([]byte{0})
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.Write").Msg(err.Error())
		return
	}
	_, err = conn.Write(comm.marshal())
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.Write").Msg(err.Error())
		return
	}
	err = encoding.SendUnit(nil, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.SendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.Flush").Msg(err.Error())
		return
	}
	err = a.rmc.SendFinished(comm.rmcID(), conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "alertHandler.handleCommitmentRequest.Flush 2").Msg(err.Error())
		return
	}
	log.Info().Msg(logging.SyncCompleted)
}

// RequestCommitment to the given preunit, from pid.
// The other party might reply indicating that they were not aware of the fork.
// In this case we send the finished alert, in a separate communication.
func (a *alertHandler) RequestCommitment(bu gomel.BaseUnit, pid uint16) error {
	log := a.log.With().Uint16(logging.PID, pid).Logger()
	conn, err := a.netserv.Dial(pid, a.timeout)
	if err != nil {
		return err
	}
	conn.TimeoutAfter(a.timeout)
	log.Info().Msg(logging.SyncStarted)
	defer conn.Close()
	err = rmc.Greet(conn, a.myPid, 0, request)
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.Greet").Msg(err.Error())
		return err
	}
	_, err = conn.Write(bu.Hash()[:])
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.Write").Msg(err.Error())
		return err
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.Flush").Msg(err.Error())
		return err
	}
	buf := make([]byte, 1)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.ReadFull").Msg(err.Error())
		return err
	}
	if buf[0] == 1 {
		a.sendFinished(bu.Creator(), pid)
		return errors.New("peer was unaware of forker")
	}
	comms, err := acquireCommitments(conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.acquireCommitments").Msg(err.Error())
		return err
	}
	_, raiser, _ := a.decodeAlertID(comms[0].rmcID(), 0)
	data, err := a.rmc.AcceptFinished(comms[0].rmcID(), raiser, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.AcceptFinished").Msg(err.Error())
		return err
	}
	proof, _ := (&forkingProof{}).unmarshal(data)
	err = a.commitments.addBatch(comms, proof, raiser)
	if err != nil {
		log.Error().Str("where", "alertHandler.RequestCommitment.addBatch").Msg(err.Error())
		return err
	}
	log.Info().Msg(logging.SyncCompleted)
	return nil
}

// acceptAlert and, if it is correct, sign it. In this case, if this is the first time we learn about this process forking,
// also raise our own alert afterwards.
func (a *alertHandler) acceptAlert(id uint64, pid uint16, conn network.Connection, log zerolog.Logger) {
	forker, _, err := a.decodeAlertID(id, pid)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptAlert.decodeAlertID").Msg(err.Error())
		return
	}
	data, err := a.rmc.AcceptData(id, pid, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptAlert.AcceptData").Msg(err.Error())
		return
	}
	proof, err := (&forkingProof{}).unmarshal(data)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptAlert.Unmarshal").Msg(err.Error())
		return
	}
	err = proof.checkCorrectness(forker, a.keys[forker])
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptAlert.checkCorrectness").Msg(err.Error())
		return
	}
	comm := proof.extractCommitment(id)
	a.commitments.add(comm, pid, forker)
	err = a.maybeSign(id, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptAlert.maybeSign").Msg(err.Error())
	} else {
		log.Info().Msg(logging.SyncCompleted)
	}
	a.Lock(forker)
	defer a.Unlock(forker)
	if a.commitments.getByParties(a.myPid, pid) == nil {
		maxes := a.dag.MaximalUnitsPerProcess().Get(forker)
		if len(maxes) == 0 {
			proof.replaceCommit(nil)
		} else {
			proof.replaceCommit(maxes[0])
		}
		a.raiseAlert(proof)
	}
}

func (a *alertHandler) maybeSign(id uint64, conn network.Connection) error {
	err := a.rmc.SendSignature(id, conn)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (a *alertHandler) acceptProof(id uint64, conn network.Connection, log zerolog.Logger) {
	err := a.rmc.AcceptProof(id, conn)
	if err != nil {
		log.Error().Str("where", "alertHandler.acceptProof.AcceptProof").Msg(err.Error())
		return
	}
}

// raiseAlert using the provided proof.
// Blocks until the RMC is successful, retrying anyone who failed to sign in the meantime.
// Should be only ran when the forker's id is locked.
func (a *alertHandler) raiseAlert(proof *forkingProof) {
	if a.commitments.getByParties(a.myPid, proof.forkerID()) != nil {
		// We already committed at some point, no reason to do it again.
		return
	}
	wg := &sync.WaitGroup{}
	gathering := &sync.WaitGroup{}
	id := a.alertID(proof.forkerID())
	data := proof.marshal()
	for pid := uint16(0); pid < a.nProc; pid++ {
		if pid == a.myPid || pid == proof.forkerID() {
			continue
		}
		wg.Add(1)
		gathering.Add(1)
		go a.sendAlert(data, id, pid, gathering, wg)
	}
	wg.Wait()
	comm := proof.extractCommitment(id)
	a.commitments.add(comm, a.myPid, proof.forkerID())
}

func (a *alertHandler) alertID(forker uint16) uint64 {
	return uint64(forker) + uint64(a.myPid)*uint64(a.nProc)
}

func (a *alertHandler) decodeAlertID(id uint64, pid uint16) (uint16, uint16, error) {
	forker, raiser := uint16(id%uint64(a.nProc)), uint16(id/uint64(a.nProc))
	if raiser != pid {
		return forker, raiser, errors.New("decoded id does not match provided id")
	}
	if raiser == forker {
		return forker, raiser, errors.New("cannot commit to own fork")
	}
	return forker, raiser, nil
}

// sendAlert to the given pid. Keeps trying until it succeeds or the whole RMC finishes successfully.
// Afterwards it waits for RMC to finish and, if it succeeded in gathering the signature,
// also sends the proof of the RMC finishing. In the optimistic case, it gathers signatures from all other processes,
// and sends the proofs also to all of them.
func (a *alertHandler) sendAlert(data []byte, id uint64, pid uint16, gathering, wg *sync.WaitGroup) {
	defer wg.Done()
	success := false
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, id).Logger()
	for a.rmc.Status(id) != rmc.Finished {
		conn, err := a.netserv.Dial(pid, a.timeout)
		if err != nil {
			continue
		}
		conn.TimeoutAfter(a.timeout)
		log.Info().Msg(logging.SyncStarted)
		err = a.attemptGather(conn, data, id, pid)
		if err != nil {
			log.Error().Str("where", "alertHandler.sendAlert.attemptGather").Msg(err.Error())
		} else {
			log.Info().Msg(logging.SyncCompleted)
			success = true
			break
		}
	}
	gathering.Done()
	gathering.Wait()
	if success {
		conn, err := a.netserv.Dial(pid, a.timeout)
		if err != nil {
			return
		}
		err = a.attemptProve(conn, id)
		if err != nil {
			log.Error().Str("where", "alertHandler.sendAlert.attemptProve").Msg(err.Error())
		}
	}
}

func (a *alertHandler) attemptGather(conn network.Connection, data []byte, id uint64, pid uint16) error {
	defer conn.Close()
	err := rmc.Greet(conn, a.myPid, id, alert)
	if err != nil {
		return err
	}
	err = a.rmc.SendData(id, data, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	_, err = a.rmc.AcceptSignature(id, pid, conn)
	if err != nil {
		return err
	}
	return nil
}

func (a *alertHandler) attemptProve(conn network.Connection, id uint64) error {
	defer conn.Close()
	err := rmc.Greet(conn, a.myPid, id, proving)
	if err != nil {
		return err
	}
	err = a.rmc.SendProof(id, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	return nil
}

// disambiguateForker uses the commitment to this unit to figure out which unit is its predecessor.
func (a *alertHandler) disambiguateForker(possibleParents []gomel.Unit, pu gomel.Preunit) (gomel.Unit, error) {
	comm := a.commitments.getByHash(pu.Hash())
	if comm == nil {
		return nil, missingCommitment("missing commitment to fork")
	}
	h := comm.getParentHash(pu.Creator())
	if h == nil {
		return nil, errors.New("too shallow commitment")
	}
	for _, u := range possibleParents {
		if *h == *u.Hash() {
			return u, nil
		}
	}
	return nil, gomel.NewUnknownParents(1)
}

// Disambiguate which of the possibleParents is the actual parent of a unit created by pid.
// Only uses local data, if some is missing a MissingDataError is returned.
func (a *alertHandler) Disambiguate(possibleParents []gomel.Unit, pu gomel.Preunit) (gomel.Unit, error) {
	if len(possibleParents) == 0 {
		return nil, nil
	}
	if len(possibleParents) == 1 {
		return possibleParents[0], nil
	}
	pid := pu.Creator()
	forker := possibleParents[0].Creator()
	if pid == forker {
		return a.disambiguateForker(possibleParents, pu)
	}
	height := possibleParents[0].Height()
	comm := a.commitments.getByParties(pid, forker)
	if comm == nil {
		return nil, missingCommitment("no commitment by this pid")
	}
	cu := comm.getUnit()
	if cu == nil {
		return nil, gomel.NewComplianceError("unit built on noncommitted parent")
	}
	u := a.dag.GetUnit(cu.Hash())
	if u == nil {
		return nil, missingCommitment("no committed unit needed for disambiguation")
	}
	if u.Height() < height {
		return nil, gomel.NewComplianceError("unit built on noncommitted parent")
	}
	for u.Height() > height {
		u = gomel.Predecessor(u)
	}
	return u, nil
}

// hasCommitmentTo checks whether we have a proof that someone committed to the provided unit.
func (a *alertHandler) hasCommitmentTo(u gomel.Unit) bool {
	comm := a.commitments.getByHash(u.Hash())
	if comm == nil {
		return false
	}
	return true
}

// IsForker checks whether the provided pid corresponds to a process for which we have a proof of forking.
func (a *alertHandler) IsForker(forker uint16) bool {
	return a.commitments.isForker(forker)
}

// Lock the alerts related to the provided pid.
func (a *alertHandler) Lock(pid uint16) {
	a.locks[pid].Lock()
}

// Unlock the alerts related to the provided pid.
func (a *alertHandler) Unlock(pid uint16) {
	a.locks[pid].Unlock()
}

// checkCommitment checks if the given unit was produced by forker and, if this is the case, whether it has required commitment.
func (a *alertHandler) checkCommitment(u gomel.Unit) error {
	if a.handleForkerUnit(u) && !a.hasCommitmentTo(u) {
		return missingCommitment("missing commitment to fork")
	}
	return nil
}

func (a *alertHandler) handleForkerUnit(u gomel.Unit) bool {
	creator := u.Creator()
	if a.IsForker(creator) {
		return true
	}
	maxes := a.dag.MaximalUnitsPerProcess().Get(creator)
	if len(maxes) == 0 {
		return false
	}
	// There can be only one, because the creator is not yet a forker.
	max := maxes[0]
	if max.Height() >= u.Height() {
		v := max
		for v.Height() > u.Height() {
			v = gomel.Predecessor(v)
		}
		proof := newForkingProof(u, v, max)
		if proof == nil {
			return false
		}
		a.raiseAlert(proof)
		return true
	}
	return false
}

// ResolveMissingCommitment checks if the given error is about missing commitment and tries to resolve it.
func (a *alertHandler) ResolveMissingCommitment(e error, u gomel.BaseUnit, source uint16) error {
	if e != nil {
		switch e.(type) {
		case *noCommitment:
			err := a.RequestCommitment(u, source)
			if err != nil {
				return err
			}
		default:
			return e
		}
	}
	return nil
}

// NewFork takes two preunits that prove that a fork happened, produces a forking proof and raises an alert.
func (a *alertHandler) NewFork(u, v gomel.Preunit) {
	if u.Creator() != v.Creator() || u.Height() != v.Height() {
		return
	}

	a.Lock(u.Creator())
	defer a.Unlock(u.Creator())

	if a.IsForker(u.Creator()) {
		return
	}

	maxes := a.dag.MaximalUnitsPerProcess().Get(u.Creator())
	// There can be only one unit in maxes, since its creator is not a forker.
	var max gomel.Unit
	if len(maxes) > 0 {
		max = maxes[0]
	}

	proof := newForkingProof(u, v, max)
	a.raiseAlert(proof)
}
