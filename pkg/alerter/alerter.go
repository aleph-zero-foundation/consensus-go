// Package alerter provides services and DAG wrappers that handle alerts raised when forks are encountered.
package alerter

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
	request
)

// Alerter allows to raise alerts and handle commitments to units.
type Alerter struct {
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

// New alerter for raising alerts and handling commitments.
func New(myPid uint16, dag gomel.Dag, keys []gomel.PublicKey, rmc *rmc.RMC, netserv network.Server, timeout time.Duration, log zerolog.Logger) *Alerter {
	nProc := uint16(len(keys))
	return &Alerter{
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
func (a *Alerter) HandleIncoming(conn network.Connection, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()
	pid, id, msgType, err := rmc.AcceptGreeting(conn)
	if err != nil {
		a.log.Error().Str("where", "Alerter.handleIncoming.AcceptGreeting").Msg(err.Error())
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
	case request:
		a.handleCommitmentRequest(conn, log)
	}
}

func (a *Alerter) produceCommitmentFor(unit gomel.Unit) (commitment, error) {
	comm := a.commitments.getByParties(a.myPid, unit.Creator())
	if comm == nil {
		return nil, errors.New("we are not aware of any forks here")
	}
	commUnit := comm.getUnit()
	if commUnit == nil {
		return nil, errors.New("we did not commit to anything")
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
			return nil, errors.New("something went very wrong")
		}
	} else {
		comm = a.commitments.getByHash(commUnit.Hash())
		for commUnit.Height() > unit.Height() {
			comm = comm.commitmentForParent(pred)
			if comm == nil {
				return nil, errors.New("failed to produce commitment for predecessor")
			}
			a.commitments.add(comm, a.myPid, pred.Creator())
			commUnit = pred
			pred = gomel.Predecessor(commUnit)
		}
		if *comm.getHash() != *unit.Hash() {
			return nil, errors.New("somehow produced commitment for wrong unit")
		}
	}
	return comm, nil
}

func (a *Alerter) handleCommitmentRequest(conn network.Connection, log zerolog.Logger) {
	var requested gomel.Hash
	_, err := io.ReadFull(conn, requested[:])
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.ReadFull").Msg(err.Error())
		return
	}
	unit := a.dag.Get([]*gomel.Hash{&requested})[0]
	if unit == nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.Get").Msg("no commitment for unit not in dag")
		return
	}
	comm := a.commitments.getByHash(&requested)
	if comm == nil {
		comm, err = a.produceCommitmentFor(unit)
		if err != nil {
			log.Error().Str("where", "Alerter.handleCommitmentRequest.produceCommitmentFor").Msg(err.Error())
			return
		}
	}
	_, err = conn.Write(comm.Marshal())
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.Write").Msg(err.Error())
		return
	}
	err = encoding.SendUnit(nil, conn)
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.SendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.Flush").Msg(err.Error())
		return
	}
	err = a.rmc.SendFinished(comm.rmcID(), conn)
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alerter.handleCommitmentRequest.Flush 2").Msg(err.Error())
		return
	}
	log.Info().Msg(logging.SyncCompleted)
}

// RequestCommitment to the unit with the given hash, from pid.
func (a *Alerter) RequestCommitment(hash *gomel.Hash, pid uint16) error {
	log := a.log.With().Uint16(logging.PID, pid).Logger()
	conn, err := a.netserv.Dial(pid, a.timeout)
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.Dial").Msg(err.Error())
		return err
	}
	conn.TimeoutAfter(a.timeout)
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)
	defer conn.Close()
	err = rmc.Greet(conn, a.myPid, 0, request)
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.Greet").Msg(err.Error())
		return err
	}
	_, err = conn.Write(hash[:])
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.Write").Msg(err.Error())
		return err
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.Flush").Msg(err.Error())
		return err
	}
	comms, err := acquireCommitments(conn)
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.acquireCommitments").Msg(err.Error())
		return err
	}
	_, raiser := a.decodeAlertID(comms[0].rmcID())
	data, err := a.rmc.AcceptFinished(comms[0].rmcID(), raiser, conn)
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.AcceptFinished").Msg(err.Error())
		return err
	}
	proof, _ := (&forkingProof{}).Unmarshal(data)
	err = a.commitments.addBatch(comms, proof, raiser)
	if err != nil {
		log.Error().Str("where", "Alerter.RequestCommitment.addBatch").Msg(err.Error())
		return err
	}
	log.Info().Msg(logging.SyncCompleted)
	return nil
}

func (a *Alerter) acceptAlert(id uint64, pid uint16, conn network.Connection, log zerolog.Logger) {
	forker, raiser := a.decodeAlertID(id)
	if raiser != pid {
		log.Error().Str("where", "Alerter.acceptAlert.decodeAlertID").Msg("decoded id does not match provided id")
		return
	}
	if raiser == forker {
		log.Error().Str("where", "Alerter.acceptAlert.decodeAlertID").Msg("cannot commit to own fork")
		return
	}
	data, err := a.rmc.AcceptData(id, pid, conn)
	if err != nil {
		log.Error().Str("where", "Alerter.acceptAlert.AcceptData").Msg(err.Error())
		return
	}
	proof, err := (&forkingProof{}).Unmarshal(data)
	if err != nil {
		log.Error().Str("where", "Alerter.acceptAlert.Unmarshal").Msg(err.Error())
		return
	}
	err = proof.checkCorrectness(forker, a.keys[forker])
	if err != nil {
		log.Error().Str("where", "Alerter.acceptAlert.checkCorrectness").Msg(err.Error())
		return
	}
	comm := proof.extractCommitment(id)
	a.commitments.add(comm, pid, forker)
	err = a.maybeSign(id, conn)
	if err != nil {
		log.Error().Str("where", "Alerter.acceptAlert.maybeSign").Msg(err.Error())
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

func (a *Alerter) maybeSign(id uint64, conn network.Connection) error {
	err := a.rmc.SendSignature(id, conn)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func (a *Alerter) acceptProof(id uint64, conn network.Connection, log zerolog.Logger) {
	err := a.rmc.AcceptProof(id, conn)
	if err != nil {
		log.Error().Str("where", "Alerter.acceptProof.AcceptProof").Msg(err.Error())
		return
	}
}

// Raise an alert using the provided proof.
func (a *Alerter) Raise(proof *forkingProof) {
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

func (a *Alerter) alertID(forker uint16) uint64 {
	return uint64(forker) + uint64(a.myPid)*uint64(a.nProc)
}

func (a *Alerter) decodeAlertID(id uint64) (uint16, uint16) {
	return uint16(id % uint64(a.nProc)), uint16(id / uint64(a.nProc))
}

func (a *Alerter) sendAlert(data []byte, id uint64, pid uint16, gathering, wg *sync.WaitGroup) {
	defer wg.Done()
	success := false
	log := a.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, id).Logger()
	for a.rmc.Status(id) != rmc.Finished {
		conn, err := a.netserv.Dial(pid, a.timeout)
		if err != nil {
			log.Error().Str("where", "Alerter.sendAlert.Dial").Msg(err.Error())
			continue
		}
		conn.TimeoutAfter(a.timeout)
		conn.SetLogger(log)
		log.Info().Msg(logging.SyncStarted)
		err = a.attemptGather(conn, data, id, pid)
		if err != nil {
			log.Error().Str("where", "Alerter.sendAlert.attemptGather").Msg(err.Error())
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
			log.Error().Str("where", "Alerter.sendAlert.Dial 2").Msg(err.Error())
			return
		}
		err = a.attemptProve(conn, id)
		if err != nil {
			log.Error().Str("where", "Alerter.sendAlert.attemptProve").Msg(err.Error())
		}
	}
}

func (a *Alerter) attemptGather(conn network.Connection, data []byte, id uint64, pid uint16) error {
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

func (a *Alerter) attemptProve(conn network.Connection, id uint64) error {
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

// CommitmentTo checks whether we are committed to the provided unit.
func (a *Alerter) CommitmentTo(u gomel.Unit) bool {
	comm := a.commitments.getByHash(u.Hash())
	if comm == nil {
		return false
	}
	comm.setUnit(u)
	return true
}

// IsForker checks whether the provided pid corresponds to a process for which we have a proof of forking.
func (a *Alerter) IsForker(forker uint16) bool {
	return a.commitments.isForker(forker)
}

// Lock the alerts related to the provided pid.
func (a *Alerter) Lock(pid uint16) {
	a.locks[pid].Lock()
}

// Unlock the alerts related to the provided pid.
func (a *Alerter) Unlock(pid uint16) {
	a.locks[pid].Unlock()
}
