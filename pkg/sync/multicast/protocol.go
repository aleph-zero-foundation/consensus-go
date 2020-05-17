package multicast

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
)

func (s *server) In() {
	conn, err := s.netserv.Listen()
	if err != nil {
		return
	}
	defer conn.Close()

	preunit, err := encoding.ReadPreunit(conn)
	if err != nil {
		s.log.Error().Str("where", "multicast.in.decode").Msg(err.Error())
		return
	}
	lg.AddingErrors(s.orderer.AddPreunits(preunit.Creator(), preunit), 1, s.log)
}

func (s *server) Out(pid uint16) {
	r, ok := <-s.requests[pid]
	if !ok {
		return
	}
	conn, err := s.netserv.Dial(pid)
	if err != nil {
		return
	}
	defer conn.Close()
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
	s.log.Info().Int(lg.Height, r.height).Uint16(lg.PID, pid).Msg(lg.SentUnit)
}
