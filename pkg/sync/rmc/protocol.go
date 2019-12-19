package rmc

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
	"gitlab.com/alephledger/validator-skeleton/pkg/network"
	rmcbox "gitlab.com/alephledger/validator-skeleton/pkg/rmc"
)

func (p *server) multicast(unit gomel.Unit) {
	id := gomel.UnitID(unit)
	data, err := encoding.EncodeUnit(unit)
	if err != nil {
		p.log.Error().Str("where", "rmcServer.Send.EncodeUnit").Msg(err.Error())
		return
	}
	p.multicastInProgress.Lock()
	signedBy := p.getCommitteeSignatures(data, id)
	p.multicastInProgress.Unlock()
	for pid, isSigned := range signedBy {
		if isSigned {
			go func(pid uint16) {
				err := p.sendProof(pid, id)
				if err != nil {
					p.log.Error().Str("where", "rmcServer.SendProof").Msg(err.Error())
				}
			}(uint16(pid))
		}
	}
}

func (p *server) sendProof(receipient uint16, id uint64) error {
	conn, err := p.netserv.Dial(receipient, p.timeout)
	if err != nil {
		return err
	}
	err = rmcbox.Greet(conn, p.pid, id, sendProof)
	if err != nil {
		return err
	}
	err = p.state.SendProof(id, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	return nil
}

// getCommitteeSignatures collects signatures of all other committee members
// on given data with given rmc id.
// It blocks until it gathers at least quorum signatures.
// It returns nProc boolean values in a slice, i-th value indicates
// weather the i-th process signed the data or not.
func (p *server) getCommitteeSignatures(data []byte, id uint64) []bool {
	signedBy := make([]bool, p.dag.NProc())
	gathering := &sync.WaitGroup{}
	for pid := uint16(0); pid < p.dag.NProc(); pid++ {
		if pid == p.pid {
			continue
		}
		gathering.Add(1)
		go func(pid uint16) {
			signedBy[pid] = p.getMemberSignature(data, id, pid, gathering)
		}(pid)
	}
	gathering.Wait()
	return signedBy
}

// getMemberSignature tries to get a signature from the given recipient on a given data with given rmc id.
// It retries until it gets a signature, or there are at least quorum signatures for this rmc-id
// gathered from different recipients.
// It returns whether it got a signature or not.
func (p *server) getMemberSignature(data []byte, id uint64, receipient uint16, gathering *sync.WaitGroup) bool {
	defer gathering.Done()
	log := p.log.With().Uint16(logging.PID, receipient).Uint64(logging.OSID, id).Logger()
	for p.state.Status(id) != rmcbox.Finished && atomic.LoadInt64(&p.quit) == 0 {
		conn, err := p.netserv.Dial(receipient, p.timeout)
		if err != nil {
			continue
		}
		conn.TimeoutAfter(p.timeout)
		log.Info().Msg(logging.SyncStarted)
		err = p.attemptGather(conn, data, id, receipient)
		if err == nil {
			log.Info().Msg(logging.SyncCompleted)
			return true
		}
		log.Error().Str("where", "sync.rmc.attemptGather").Msg(err.Error())
	}
	return false
}

func (p *server) attemptGather(conn network.Connection, data []byte, id uint64, receipient uint16) error {
	defer conn.Close()
	err := rmcbox.Greet(conn, p.pid, id, sendData)
	if err != nil {
		return err
	}
	err = p.state.SendData(id, data, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	_, err = p.state.AcceptSignature(id, receipient, conn)
	if err != nil {
		return err
	}
	return nil
}

func (p *server) sendProve(conn network.Connection, id uint64) error {
	defer conn.Close()
	err := rmcbox.Greet(conn, p.pid, id, sendProof)
	if err != nil {
		return err
	}
	err = p.state.SendProof(id, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (p *server) in() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := rmcbox.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "rmc.in.AcceptGreeting").Msg(err.Error())
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint64(logging.ISID, id).Logger()
	log.Info().Msg(logging.SyncStarted)

	switch msgType {
	case sendData:
		p.acceptData(id, pid, conn, log)
	case sendProof:
		if p.acceptProof(id, conn, log) {
			pu, err := encoding.DecodePreunit(p.state.Data(id))
			if err != nil {
				log.Error().Str("where", "rmc.in.DecodePreunit3").Msg(err.Error())
				return
			}
			add.Unit(p.adder, pu, pu.Creator(), "rmc.in", log)
		}
	case requestFinished:
		err := p.state.SendFinished(id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.SendFinished").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.in.Flush4").Msg(err.Error())
			return
		}

	}
	log.Info().Msg(logging.SyncCompleted)
}

func (p *server) acceptProof(id uint64, conn network.Connection, log zerolog.Logger) bool {
	err := p.state.AcceptProof(id, conn)
	if err != nil {
		log.Error().Str("where", "rmc.acceptProof.AcceptProof").Msg(err.Error())
		return false
	}
	return true
}

func (p *server) acceptData(id uint64, sender uint16, conn network.Connection, log zerolog.Logger) {
	data, err := p.state.AcceptData(id, sender, conn)
	if err != nil {
		log.Error().Str("where", "rmc.in.AcceptData").Msg(err.Error())
		return
	}
	pu, err := encoding.DecodePreunit(data)
	if err != nil {
		log.Error().Str("where", "rmc.in.DecodePreunit").Msg(err.Error())
		return
	}
	if id != gomel.UnitID(pu) {
		log.Error().Str("what", "wrong preunit id").Msg(err.Error())
		return
	}
	err = p.state.SendSignature(id, conn)
	if err != nil {
		log.Error().Str("where", "rmc.in.SendSignature").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "rmc.in.Flush").Msg(err.Error())
		return
	}
}
