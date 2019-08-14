package fetch

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

type protocol struct {
	dag      gomel.Dag
	rs       gomel.RandomSource
	pid      uint16
	requests chan gomel.Preunit
	dialer   network.Dialer
	listener network.Listener
	timeout  time.Duration
	log      zerolog.Logger
}

// NewProtocol returns an instance of protocol for fetching all the missing
// ancestors of a unit received from rmc
func NewProtocol(pid uint16, dag gomel.Dag, rs gomel.RandomSource, requests chan gomel.Preunit, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) gsync.Protocol {
	return &protocol{
		dag:      dag,
		rs:       rs,
		pid:      pid,
		requests: requests,
		dialer:   dialer,
		listener: listener,
		timeout:  timeout,
		log:      log,
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	u, err := receiveUnit(conn, p.dag)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.getPreunit").Msg(err.Error())
		return
	}
	heights, err := receiveHeights(conn, p.dag.NProc())
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.getHeights").Msg(err.Error())
		return
	}
	units := getUnits(p.dag, u, heights)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.getUnits").Msg(err.Error())
		return
	}
	err = sendUnits(conn, units)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.sendUnits").Msg(err.Error())
		return
	}
}

func (p *protocol) Out() {
	pu, isOpen := <-p.requests
	if !isOpen {
		return
	}
	pid := uint16(pu.Creator())
	conn, err := p.dialer.Dial(pid)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = sendPu(conn, pu)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.sendPu").Msg(err.Error())
		return
	}
	err = sendHeights(conn, p.dag)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.sendHeights").Msg(err.Error())
		return
	}
	pus, err := receivePreunits(conn)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.receivePreunits").Msg(err.Error())
		return
	}
	err = addUnits(pus, pu, p.dag, p.rs)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.addUnits").Msg(err.Error())
		return
	}
}

func receiveUnit(conn network.Connection, dag gomel.Dag) (gomel.Unit, error) {
	hash := &gomel.Hash{}
	_, err := io.ReadFull(conn, hash[:])
	if err != nil {
		return nil, err
	}
	units := dag.Get([]*gomel.Hash{hash})
	if units[0] == nil {
		return nil, errors.New("Unknown unit")
	}
	return units[0], nil
}

func receiveHeights(conn network.Connection, nProc int) ([]int, error) {
	buf := make([]byte, 4*nProc)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	result := make([]int, nProc)
	for pid := range result {
		result[pid] = int(binary.LittleEndian.Uint32(buf[(4 * pid):(4*pid + 4)]))
	}
	return result, nil
}

func getUnits(dag gomel.Dag, u gomel.Unit, heights []int) []gomel.Unit {
	result := []gomel.Unit{}
	for pid, units := range u.Floor() {
		if len(units) == 0 {
			continue
		}
		var err error
		v := units[0]
		for v.Height() >= heights[pid] {
			result = append(result, v)
			v, err = gomel.Predecessor(v)
			if err != nil {
				break
			}
		}
	}
	return result
}

func sendUnits(conn network.Connection, units []gomel.Unit) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(units)))
	conn.Write(buf)

	encoder := custom.NewEncoder(conn)
	for _, u := range units {
		err := encoder.EncodeUnit(u)
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func sendPu(conn network.Connection, pu gomel.Preunit) error {
	_, err := conn.Write(pu.Hash()[:])
	if err != nil {
		return err
	}
	return conn.Flush()
}

func sendHeights(conn network.Connection, dag gomel.Dag) error {
	heights := make([]int, 0, dag.NProc())
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		if len(units) == 0 {
			heights = append(heights, -1)
			return true
		}
		heights = append(heights, units[0].Height())
		return true
	})
	buf := make([]byte, 4)
	for _, h := range heights {
		// we are sending first height that is missing per pid
		binary.LittleEndian.PutUint32(buf, uint32(h+1))
		conn.Write(buf)
	}
	return conn.Flush()
}

func receivePreunits(conn network.Connection) ([]gomel.Preunit, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	nUnits := binary.LittleEndian.Uint32(buf)
	result := make([]gomel.Preunit, nUnits)
	decoder := custom.NewDecoder(conn)
	for i := range result {
		result[i], err = decoder.DecodePreunit()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func addUnits(pus []gomel.Preunit, pu gomel.Preunit, dag gomel.Dag, rs gomel.RandomSource) error {
	preunitByHash := make(map[gomel.Hash]gomel.Preunit)
	for _, p := range pus {
		preunitByHash[*p.Hash()] = p
	}
	var dfsAdd func(p gomel.Preunit) bool
	// dfsAdd tries to add given preunit p to the dag,
	// using the received set of preunits: preunitByHash
	// to lookup missing parents.
	//
	// We are calling this function only on "trusted" preunits
	// i.e. preunits that either comes from RMC
	// or are parents of trusted unit.
	dfsAdd = func(p gomel.Preunit) bool {
		parents := dag.Get(p.Parents())
		for i, parent := range parents {
			if parent == nil {
				parentOutOfDag := p.Parents()[i]
				if received, ok := preunitByHash[*parentOutOfDag]; ok {
					if !dfsAdd(received) {
						return false
					}
				}
			}
		}
		var wg sync.WaitGroup
		wg.Add(1)
		ok := true
		dag.AddUnit(p, rs, func(_ gomel.Preunit, u gomel.Unit, err error) {
			defer wg.Done()
			if err != nil {
				switch err.(type) {
				case *gomel.DuplicateUnit:
				default:
					ok = false
				}
			}
		})
		wg.Wait()
		if ok {
			return true
		}
		return false
	}
	success := dfsAdd(pu)
	if !success {
		return errors.New("fetched info wasn't enough to add the preunit")
	}
	return nil
}
