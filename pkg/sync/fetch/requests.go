package fetch

import (
	"encoding/binary"
	"io"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type request struct {
	pid     uint16
	heights []int
}

func sendRequests(conn network.Connection, heights []int) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(heights)))
	_, err := conn.Write(buf)
	if err != nil {
		return err
	}
	for _, h := range heights {
		if h == -1 {
			binary.LittleEndian.PutUint32(buf, math.MaxUint32)
		} else {
			binary.LittleEndian.PutUint32(buf, uint32(h))
		}
		_, err := conn.Write(buf)
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func receiveRequests(conn network.Connection) ([]int, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	result := make([]int, binary.LittleEndian.Uint32(buf))
	for i := range result {
		_, err := io.ReadFull(conn, buf)
		if err != nil {
			return nil, err
		}
		h := binary.LittleEndian.Uint32(buf)
		if h == math.MaxUint32 {
			result[i] = -1
		} else {
			result[i] = int(h)
		}
	}
	return result, nil
}

func getUnits(dag gomel.Dag, heights []int) ([]gomel.Unit, error) {
	result := []gomel.Unit{}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			for u != nil && u.Height() > heights[u.Creator()] {
				result = append(result, u)
				u = gomel.Predecessor(u)
			}
		}
		return true
	})
	return result, nil
}
