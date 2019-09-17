package rmc

import (
	"errors"
	"io"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func (p *server) in() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.in.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := rmcbox.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "rmc.in.AcceptGreeting").Msg(err.Error())
		return
	}
	switch msgType {
	case sendData:
		data, err := p.state.AcceptData(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.AcceptData").Msg(err.Error())
			return
		}
		pu, err := custom.DecodePreunit(data)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.DecodePreunit").Msg(err.Error())
			return
		}
		knownPredecessor, err := checkCompliance(pu, id, pid, p.dag)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.checkCompliance").Msg(err.Error())
			return
		}
		if !knownPredecessor {
			conn.Write([]byte{1})
			err := conn.Flush()
			if err != nil {
				p.log.Error().Str("where", "rmc.in.Flush").Msg(err.Error())
				return
			}
			data, err := p.state.AcceptFinished(predecessorID(id, p.dag.NProc()), pid, conn)
			if err != nil {
				p.log.Error().Str("where", "rmc.in.AcceptFinished").Msg(err.Error())
				return
			}
			predecessor, err := custom.DecodePreunit(data)
			if err != nil {
				p.log.Error().Str("where", "rmc.in.DecodePreunit2").Msg(err.Error())
				return
			}
			if *pu.Parents()[0] != *predecessor.Hash() {
				p.log.Error().Str("where", "rmc.in.").Msg("wrong unit height")
				return
			}
			err = add.Unit(p.dag, p.randomSource, predecessor, p.fallback, p.log)
			if err != nil {
				p.log.Error().Str("where", "rmc.in.AddPredecessor").Msg(err.Error())
				return
			}
		} else {
			conn.Write([]byte{0})
			err := conn.Flush()
			if err != nil {
				p.log.Error().Str("where", "rmc.in.Flush2").Msg(err.Error())
				return
			}
		}
		err = p.state.SendSignature(id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.SendSignature").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.in.Flush3").Msg(err.Error())
			return
		}

	case sendFinished:
		_, err := p.state.AcceptFinished(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.AcceptFinished2").Msg(err.Error())
			return
		}
		pu, err := custom.DecodePreunit(p.state.Data(id))
		if err != nil {
			p.log.Error().Str("where", "rmc.in.DecodePreunit3").Msg(err.Error())
			return
		}
		err = add.Unit(p.dag, p.randomSource, pu, p.fallback, p.log)
		if err != nil {
			p.log.Error().Str("where", "rmc.in.AddUnit").Msg(err.Error())
			return
		}
	}
}

func (p *server) out(pid uint16) {
	r, ok := <-p.requests[pid]
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(pid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = rmcbox.Greet(conn, p.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "rmc.out.Greet").Msg(err.Error())
		return
	}

	switch r.msgType {
	case sendData:
		err := p.state.SendData(r.id, r.data, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.SendData").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.out.Flush").Msg(err.Error())
			return
		}

		requestPredecessor, err := readSingleByte(conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.readSingleByte").Msg(err.Error())
			return
		}
		if requestPredecessor == byte(1) {
			err := p.state.SendFinished(predecessorID(r.id, p.dag.NProc()), conn)
			if err != nil {
				p.log.Error().Str("where", "rmc.out.SendFinished").Msg(err.Error())
				return
			}
			err = conn.Flush()
			if err != nil {
				p.log.Error().Str("where", "rmc.out.Flush2").Msg(err.Error())
				return
			}
		}
		finished, err := p.state.AcceptSignature(r.id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.AcceptSignature").Msg(err.Error())
			return
		}
		if finished {
			for _, i := range rand.Perm(int(p.dag.NProc())) {
				if i == int(p.pid) {
					continue
				}
				p.requests[i] <- newRequest(r.id, r.data, sendFinished)
			}
			//p.canMulticast.Unlock()
		}

	case sendFinished:
		err := p.state.SendFinished(r.id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.SendFinished").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.out.Flush3").Msg(err.Error())
			return
		}
	}
}

// checkCompliance checks whether the rmc id corresponds to the creator and height of the preunit.
// It returns boolean value whether we have enough information to check the preunit
// (i.e. we are able to compute it's height using the current local view).
// The second returned value is the result of the check.
func checkCompliance(pu gomel.Preunit, id uint64, pid uint16, dag gomel.Dag) (bool, error) {
	creator, height := decodeUnitID(id, dag.NProc())
	if pu.Creator() != pid || pu.Creator() != creator {
		return true, errors.New("wrong unit creator")
	}
	if len(pu.Parents()) == 0 {
		if height != 0 {
			return true, errors.New("wrong unit height")
		}
		return true, nil
	}
	predecessor := dag.Get([]*gomel.Hash{pu.Parents()[0]})[0]
	if predecessor != nil {
		if height != predecessor.Height()+1 {
			return true, errors.New("wrong unit height")
		}
		return true, nil
	}
	return false, nil
}

func readSingleByte(r io.Reader) (byte, error) {
	var buf [1]byte
	_, err := r.Read(buf[:])
	return buf[0], err
}
