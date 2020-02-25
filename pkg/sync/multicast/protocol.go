package multicast

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

func (s *server) In() {
	conn, err := s.netserv.Listen(s.conf.Timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(s.conf.Timeout)

	preunit, err := encoding.ReadPreunit(conn)
	if err != nil {
		s.log.Error().Str("where", "multicast.in.decode").Msg(err.Error())
		return
	}
	s.orderer.AddPreunits(preunit.Creator(), preunit)
}

func (s *server) Out(pid uint16) {
	r, ok := <-s.requests[pid]
	if !ok {
		return
	}
	conn, err := s.netserv.Dial(pid, s.conf.Timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(s.conf.Timeout)
	_, err = conn.Write(r.encUnit)
	if err != nil {
		s.log.Error().Str("where", "multicast.out.sendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		s.log.Error().Str("where", "multicast.out.flush").Msg(err.Error())
		return
	}
	s.log.Info().Int(logging.Height, r.height).Uint16(logging.PID, pid).Msg(logging.UnitBroadcasted)
}
