// Package forking provides services and DAG wrappers that handle alerts raised when forks are encountered.
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
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

const (
	sending byte = iota
	proving
	finished
	request
)

var missingCommitmentToForkError = gomel.NewMissingDataError("commitment to fork")

// Alert allows to raise alerts and handle commitments to units.
type Alert struct {
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

// NewAlert for raising and handling commitments.
func NewAlert(myPid uint16, dag gomel.Dag, keys []gomel.PublicKey, rmc *rmc.RMC, netserv network.Server, timeout time.Duration, log zerolog.Logger) *Alert {
	nProc := uint16(len(keys))
	return &Alert{
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
}

// HandleIncoming connection, either accepting an alert or responding to a commitment request.
func (a *Alert) HandleIncoming(conn network.Connection, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()
	pid, id, msgType, err := rmc.AcceptGreeting(conn)
	if err != nil {
		a.log.Error().Str("where", "Alert.handleIncoming.AcceptGreeting").Msg(err.Error())
		return
	}
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.ISID, id).Logger()
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)

	switch msgType {
	case sending:
		a.acceptAlert(id, pid, conn, log)
	case proving:
		a.acceptProof(id, conn, log)
	case finished:
		a.acceptFinished(id, pid, conn, log)
	case request:
		a.handleCommitmentRequest(conn, log)
	}
}

func (a *Alert) acceptFinished(id uint64, pid uint16, conn network.Connection, log zerolog.Logger) {
	forker, _, err := a.decodeAlertID(id, pid)
	if err != nil {
		log.Error().Str("where", "Alert.acceptFinished.decodeAlertID").Msg(err.Error())
		return
	}
	data, err := a.rmc.AcceptFinished(id, pid, conn)
	if err != nil {
		log.Error().Str("where", "Alert.acceptFinished.AcceptData").Msg(err.Error())
		return
	}
	proof, err := (&forkingProof{}).Unmarshal(data)
	if err != nil {
		log.Error().Str("where", "Alert.acceptFinished.Unmarshal").Msg(err.Error())
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
		a.Raise(proof)
	}
}

func (a *Alert) sendFinished(id uint64, pid uint16) {
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, id).Logger()
	conn, err := a.netserv.Dial(pid, a.timeout)
	if err != nil {
		log.Error().Str("where", "Alert.sendFinished.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(a.timeout)
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)
	err = rmc.Greet(conn, a.myPid, id, finished)
	if err != nil {
		log.Error().Str("where", "Alert.sendFinished.Greet").Msg(err.Error())
		return
	}
	err = a.rmc.SendFinished(id, conn)
	if err != nil {
		log.Error().Str("where", "Alert.sendFinished.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alert.sendFinished.Flush").Msg(err.Error())
	}
}

func (a *Alert) produceCommitmentFor(unit gomel.Unit) (commitment, error) {
	comm := a.commitments.getByParties(a.myPid, unit.Creator())
	if comm == nil {
		return nil, errors.New("we are not aware of any forks here")
	}
	pu := comm.getUnit()
	if pu == nil {
		return nil, errors.New("we did not commit to anything")
	}
	commUnit := a.dag.Get([]*gomel.Hash{pu.Hash()})[0]
	if commUnit == nil {
		return nil, errors.New("we do not have the unit we committed to")
	}
	pred := gomel.Predecessor(commUnit)
	for pred != nil && a.CommitmentTo(pred) {
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

func (a *Alert) handleCommitmentRequest(conn network.Connection, log zerolog.Logger) {
	var requested gomel.Hash
	_, err := io.ReadFull(conn, requested[:])
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.ReadFull").Msg(err.Error())
		return
	}
	unit := a.dag.Get([]*gomel.Hash{&requested})[0]
	if unit == nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.Get").Msg("no commitment for unit not in dag")
		return
	}
	// We always want to send one commitment more if we can, so that we send the parents' hashes to add unit.
	if pred := gomel.Predecessor(unit); pred != nil {
		unit = pred
	}
	comm := a.commitments.getByHash(&requested)
	if comm == nil {
		if !a.IsForker(unit.Creator()) {
			log.Error().Str("where", "Alert.handleCommitmentRequest.getByHash").Msg("we were not aware there was a fork")
			_, err = conn.Write([]byte{1})
			if err != nil {
				log.Error().Str("where", "Alert.handleCommitmentRequest.Write").Msg(err.Error())
				return
			}
			err = conn.Flush()
			if err != nil {
				log.Error().Str("where", "Alert.handleCommitmentRequest.Flush").Msg(err.Error())
			}
			return
		}
		_, err = conn.Write([]byte{0})
		if err != nil {
			log.Error().Str("where", "Alert.handleCommitmentRequest.Write").Msg(err.Error())
			return
		}
		comm, err = a.produceCommitmentFor(unit)
		if err != nil {
			log.Error().Str("where", "Alert.handleCommitmentRequest.produceCommitmentFor").Msg(err.Error())
			return
		}
	}
	_, err = conn.Write(comm.marshal())
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.Write").Msg(err.Error())
		return
	}
	err = encoding.SendUnit(nil, conn)
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.SendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.Flush").Msg(err.Error())
		return
	}
	err = a.rmc.SendFinished(comm.rmcID(), conn)
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alert.handleCommitmentRequest.Flush 2").Msg(err.Error())
		return
	}
	log.Info().Msg(logging.SyncCompleted)
}

// RequestCommitment to the unit with the given hash, from pid.
func (a *Alert) RequestCommitment(hash *gomel.Hash, pid uint16) error {
	log := a.log.With().Uint16(logging.PID, pid).Logger()
	conn, err := a.netserv.Dial(pid, a.timeout)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.Dial").Msg(err.Error())
		return err
	}
	conn.TimeoutAfter(a.timeout)
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)
	defer conn.Close()
	err = rmc.Greet(conn, a.myPid, 0, request)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.Greet").Msg(err.Error())
		return err
	}
	_, err = conn.Write(hash[:])
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.Write").Msg(err.Error())
		return err
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.Flush").Msg(err.Error())
		return err
	}
	buf := make([]byte, 1)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.ReadFull").Msg(err.Error())
		return err
	}
	if buf[0] == 1 {
		return errors.New("peer was unaware of forker")
	}
	comms, err := acquireCommitments(conn)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.acquireCommitments").Msg(err.Error())
		return err
	}
	_, raiser, _ := a.decodeAlertID(comms[0].rmcID(), 0)
	data, err := a.rmc.AcceptFinished(comms[0].rmcID(), raiser, conn)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.AcceptFinished").Msg(err.Error())
		return err
	}
	proof, _ := (&forkingProof{}).Unmarshal(data)
	err = a.commitments.addBatch(comms, proof, raiser)
	if err != nil {
		log.Error().Str("where", "Alert.RequestCommitment.addBatch").Msg(err.Error())
		return err
	}
	log.Info().Msg(logging.SyncCompleted)
	return nil
}

func (a *Alert) acceptAlert(id uint64, pid uint16, conn network.Connection, log zerolog.Logger) {
	forker, _, err := a.decodeAlertID(id, pid)
	if err != nil {
		log.Error().Str("where", "Alert.acceptAlert.decodeAlertID").Msg(err.Error())
		return
	}
	data, err := a.rmc.AcceptData(id, pid, conn)
	if err != nil {
		log.Error().Str("where", "Alert.acceptAlert.AcceptData").Msg(err.Error())
		return
	}
	proof, err := (&forkingProof{}).Unmarshal(data)
	if err != nil {
		log.Error().Str("where", "Alert.acceptAlert.Unmarshal").Msg(err.Error())
		return
	}
	err = proof.checkCorrectness(forker, a.keys[forker])
	if err != nil {
		log.Error().Str("where", "Alert.acceptAlert.checkCorrectness").Msg(err.Error())
		return
	}
	comm := proof.extractCommitment(id)
	a.commitments.add(comm, pid, forker)
	err = a.maybeSign(id, conn)
	if err != nil {
		log.Error().Str("where", "Alert.acceptAlert.maybeSign").Msg(err.Error())
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
		a.Raise(proof)
	}
}

func (a *Alert) maybeSign(id uint64, conn network.Connection) error {
	err := a.rmc.SendSignature(id, conn)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (a *Alert) acceptProof(id uint64, conn network.Connection, log zerolog.Logger) {
	err := a.rmc.AcceptProof(id, conn)
	if err != nil {
		log.Error().Str("where", "Alert.acceptProof.AcceptProof").Msg(err.Error())
		return
	}
}

// Raise an alert using the provided proof.
func (a *Alert) Raise(proof *forkingProof) {
	if a.commitments.getByParties(a.myPid, proof.forkerID()) != nil {
		// We already committed at some point, no reason to do it again.
		return
	}
	wg := &sync.WaitGroup{}
	gathering := &sync.WaitGroup{}
	id := a.alertID(proof.forkerID())
	data := proof.Marshal()
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

func (a *Alert) alertID(forker uint16) uint64 {
	return uint64(forker) + uint64(a.myPid)*uint64(a.nProc)
}

func (a *Alert) decodeAlertID(id uint64, pid uint16) (uint16, uint16, error) {
	forker, raiser := uint16(id%uint64(a.nProc)), uint16(id/uint64(a.nProc))
	if raiser != pid {
		return forker, raiser, errors.New("decoded id does not match provided id")
	}
	if raiser == forker {
		return forker, raiser, errors.New("cannot commit to own fork")
	}
	return forker, raiser, nil
}

func (a *Alert) sendAlert(data []byte, id uint64, pid uint16, gathering, wg *sync.WaitGroup) {
	defer wg.Done()
	success := false
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, id).Logger()
	for a.rmc.Status(id) != rmc.Finished {
		conn, err := a.netserv.Dial(pid, a.timeout)
		if err != nil {
			log.Error().Str("where", "Alert.sendAlert.Dial").Msg(err.Error())
			continue
		}
		conn.TimeoutAfter(a.timeout)
		conn.SetLogger(log)
		log.Info().Msg(logging.SyncStarted)
		err = a.attemptGather(conn, data, id, pid)
		if err != nil {
			log.Error().Str("where", "Alert.sendAlert.attemptGather").Msg(err.Error())
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
			log.Error().Str("where", "Alert.sendAlert.Dial 2").Msg(err.Error())
			return
		}
		err = a.attemptProve(conn, id)
		if err != nil {
			log.Error().Str("where", "Alert.sendAlert.attemptProve").Msg(err.Error())
		}
	}
}

func (a *Alert) attemptGather(conn network.Connection, data []byte, id uint64, pid uint16) error {
	defer conn.Close()
	err := rmc.Greet(conn, a.myPid, id, sending)
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

func (a *Alert) attemptProve(conn network.Connection, id uint64) error {
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

const noncommittedParent = "unit built on noncommitted parent"

func (a *Alert) disambiguateForker(possibleParents []gomel.Unit, pu gomel.Preunit) (gomel.Unit, error) {
	comm := a.commitments.getByHash(pu.Hash())
	if comm == nil {
		return nil, missingCommitmentToForkError
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
func (a *Alert) Disambiguate(possibleParents []gomel.Unit, pu gomel.Preunit) (gomel.Unit, error) {
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
		return nil, gomel.NewMissingDataError("no commitment by this pid")
	}
	cu := comm.getUnit()
	if cu == nil {
		return nil, gomel.NewComplianceError(noncommittedParent)
	}
	u := a.dag.Get([]*gomel.Hash{cu.Hash()})[0]
	if u == nil {
		return nil, gomel.NewMissingDataError("no committed unit needed for disambiguation")
	}
	if u.Height() < height {
		return nil, gomel.NewComplianceError(noncommittedParent)
	}
	for u.Height() > height {
		u = gomel.Predecessor(u)
	}
	return u, nil
}

// CommitmentTo checks whether we are committed to the provided unit.
func (a *Alert) CommitmentTo(u gomel.Unit) bool {
	comm := a.commitments.getByHash(u.Hash())
	if comm == nil {
		return false
	}
	return true
}

// IsForker checks whether the provided pid corresponds to a process for which we have a proof of forking.
func (a *Alert) IsForker(forker uint16) bool {
	return a.commitments.isForker(forker)
}

// Lock the alerts related to the provided pid.
func (a *Alert) Lock(pid uint16) {
	a.locks[pid].Lock()
}

// Unlock the alerts related to the provided pid.
func (a *Alert) Unlock(pid uint16) {
	a.locks[pid].Unlock()
}
