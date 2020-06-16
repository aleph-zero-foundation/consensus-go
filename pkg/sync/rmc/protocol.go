package rmc

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/rmcbox"
)

func (s *server) multicast(unit gomel.Unit) {
	id := gomel.UnitID(unit)
	data, err := encoding.EncodeUnit(unit)
	if err != nil {
		s.log.Error().Str("where", "rmcServer.Send.EncodeUnit").Msg(err.Error())
		return
	}
	s.multicastInProgress.Lock()
	s.getCommitteeSignatures(data, id)
	s.multicastInProgress.Unlock()
	var wg sync.WaitGroup
	for pid := uint16(0); pid < s.nProc; pid++ {
		if pid == s.pid {
			continue
		}
		wg.Add(1)
		go func(pid uint16) {
			defer wg.Done()
			err := s.sendProof(pid, id)
			if err != nil {
				s.log.Error().Str("where", "rmcServer.SendProof").Msg(err.Error())
			}
		}(pid)

	}
	wg.Wait()
}

func (s *server) sendProof(recipient uint16, id uint64) error {
	conn, err := s.netserv.Dial(recipient)
	if err != nil {
		return err
	}
	err = rmcbox.Greet(conn, s.pid, id, msgSendProof)
	if err != nil {
		return err
	}
	err = s.state.SendProof(id, conn)
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
func (s *server) getCommitteeSignatures(data []byte, id uint64) []bool {
	signedBy := make([]bool, s.nProc)
	gathering := &sync.WaitGroup{}
	for pid := uint16(0); pid < s.nProc; pid++ {
		if pid == s.pid {
			continue
		}
		gathering.Add(1)
		go func(pid uint16) {
			defer gathering.Done()
			signedBy[pid] = s.getMemberSignature(data, id, pid)
		}(pid)
	}
	gathering.Wait()
	return signedBy
}

// getMemberSignature tries to get a signature from the given recipient on a given data with given rmc id.
// It retries until it gets a signature, or there are at least quorum signatures for this rmc-id
// gathered from different recipients.
// It returns whether it got a signature or not.
func (s *server) getMemberSignature(data []byte, id uint64, recipient uint16) bool {
	log := s.log.With().Uint16(lg.PID, recipient).Uint64(lg.OSID, id).Logger()
	for s.state.Status(id) != rmcbox.Finished {
		conn, err := s.netserv.Dial(recipient)
		if err != nil {
			log.Error().Str("where", "rmc.getMemberSignature.Dial").Msg(err.Error())
			time.Sleep(50 * time.Millisecond)
			continue
		}

		log.Info().Msg(lg.SyncStarted)
		err = s.attemptGather(conn, data, id, recipient)
		if err != nil {
			log.Error().Str("where", "rmc.attemptGather").Msg(err.Error())
			time.Sleep(50 * time.Millisecond)
			continue
		}
		log.Info().Msg(lg.SyncCompleted)
		return true
	}
	return false
}

func (s *server) attemptGather(conn network.Connection, data []byte, id uint64, recipient uint16) error {
	defer conn.Close()
	err := rmcbox.Greet(conn, s.pid, id, msgSendData)
	if err != nil {
		return err
	}
	err = s.state.SendData(id, data, conn)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	_, err = s.state.AcceptSignature(id, recipient, conn)
	if err != nil {
		return err
	}
	return nil
}

func (s *server) in() {
	conn, err := s.netserv.Listen()
	if err != nil {
		return
	}
	defer conn.Close()

	pid, id, msgType, err := rmcbox.AcceptGreeting(conn)
	if err != nil {
		s.log.Error().Str("where", "rmc.in.AcceptGreeting").Msg(err.Error())
		return
	}
	log := s.log.With().Uint16(lg.PID, pid).Uint64(lg.ISID, id).Logger()
	log.Info().Msg(lg.SyncStarted)

	switch msgType {
	case msgSendData:
		s.acceptData(id, pid, conn, log)

	case msgSendProof:
		if s.acceptProof(id, conn, log) {
			pu, err := encoding.DecodePreunit(s.state.Data(id))
			if err != nil {
				log.Error().Str("where", "rmc.in.DecodePreunit").Msg(err.Error())
				return
			}
			lg.AddingErrors(s.orderer.AddPreunits(pu.Creator(), pu), 1, log)
		}

	case msgRequestFinished:
		s.sendFinished(id, conn, log)
	}
	log.Info().Msg(lg.SyncCompleted)
}

func (s *server) acceptProof(id uint64, conn network.Connection, log zerolog.Logger) bool {
	err := s.state.AcceptProof(id, conn)
	if err != nil {
		log.Error().Str("where", "rmc.acceptProof.AcceptProof").Msg(err.Error())
		return false
	}
	return true
}

func (s *server) acceptData(id uint64, sender uint16, conn network.Connection, log zerolog.Logger) {
	data, err := s.state.AcceptData(id, sender, conn)
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
		log.Error().Str("what", "wrong preunit id").Msg(fmt.Sprintf("wrong preunit id - expected %d, received %d", id, gomel.UnitID(pu)))
		return
	}
	err = s.state.SendSignature(id, conn)
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

func (s *server) sendFinished(id uint64, conn network.Connection, log zerolog.Logger) {
	if s.state.Status(id) != rmcbox.Finished {
		log.Error().Str("where", "rmc.in.SendFinished").Msg("requested to send finished, but we are not the finished state yet")
		return
	}
	err := s.state.SendFinished(id, conn)
	if err != nil {
		log.Error().Str("where", "rmc.in.SendFinished").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "rmc.in.Flush").Msg(err.Error())
		return
	}
}
