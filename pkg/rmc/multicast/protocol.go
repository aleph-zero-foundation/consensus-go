package multicast

import (
	"errors"
	"io"
	"time"

	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

type protocol struct {
	pid          uint16
	dag          gomel.Dag
	requests     chan *Request
	state        *rmc.RMC
	accepted     chan []byte
	canMulticast *sync.Mutex
	netserv      network.Server
	timeout      time.Duration
	log          zerolog.Logger
}

func newProtocol(pid uint16, dag gomel.Dag, requests chan *Request, state *rmc.RMC, canMulticast *sync.Mutex, accepted chan []byte, netserv network.Server, timeout time.Duration, log zerolog.Logger) *protocol {
	return &protocol{
		pid:          pid,
		dag:          dag,
		requests:     requests,
		state:        state,
		canMulticast: canMulticast,
		accepted:     accepted,
		netserv:      netserv,
		timeout:      timeout,
		log:          log,
	}
}

func (p *protocol) In() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := rmc.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
		return
	}
	switch msgType {
	case sendData:
		data, err := p.state.AcceptData(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		pu, err := DecodePreunit(data)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		knownPredecessor, err := checkCompliance(pu, id, pid, p.dag)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		if !knownPredecessor {
			conn.Write([]byte{1})
			conn.Flush()
			data, err := p.state.AcceptFinished(predecessorID(id, p.dag.NProc()), pid, conn)
			if err != nil {
				p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
				return
			}
			p.accepted <- data
			predecessor, err := DecodePreunit(data)
			if err != nil {
				p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
				return
			}
			if *pu.Parents()[0] != *predecessor.Hash() {
				p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg("wrong unit height")
				return
			}
		} else {
			conn.Write([]byte{0})
			conn.Flush()
		}

		err = p.state.SendSignature(id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		conn.Flush()
	case sendFinished:
		_, err := p.state.AcceptFinished(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		p.accepted <- p.state.Data(id)
	}
}

func (p *protocol) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(r.pid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = rmc.Greet(conn, p.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
		return
	}

	switch r.msgType {
	case sendData:
		err := p.state.SendData(r.id, r.data, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		conn.Flush()

		requestPredecessor, err := readSingleByte(conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		if requestPredecessor == byte(1) {
			err := p.state.SendFinished(predecessorID(r.id, p.dag.NProc()), conn)
			if err != nil {
				p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
				return
			}
			conn.Flush()
		}
		finished, err := p.state.AcceptSignature(r.id, r.pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		if finished {
			for i := 0; i < p.dag.NProc(); i++ {
				if uint16(i) == p.pid {
					continue
				}
				p.requests <- NewRequest(r.id, uint16(i), r.data, sendFinished)
			}
			p.canMulticast.Unlock()
		}
	case sendFinished:
		err := p.state.SendFinished(r.id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		conn.Flush()
	}
}

func checkCompliance(pu gomel.Preunit, id uint64, pid uint16, dag gomel.Dag) (bool, error) {
	if pu.Creator() != int(pid) {
		return false, errors.New("wrong unit creator")
	}
	creator, height := decodeUnitID(id, dag.NProc())
	if pu.Creator() != int(pid) || pu.Creator() != creator {
		return false, errors.New("wrong unit creator")
	}
	if len(pu.Parents()) == 0 {
		if height != 0 {
			return false, errors.New("wrong unit height")
		}
		return true, nil
	}
	predecessor := dag.Get([]*gomel.Hash{pu.Parents()[0]})[0]
	if predecessor != nil {
		if height != predecessor.Height()+1 {
			return false, errors.New("wrong unit height")
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

func predecessorID(id uint64, nProc int) uint64 {
	return id - uint64(nProc)
}
