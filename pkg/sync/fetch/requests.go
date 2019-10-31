package fetch

import (
	"encoding/binary"
	"errors"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type request struct {
	pid     uint16
	unitIDs []uint64
}

func sendRequests(conn network.Connection, unitIDs []uint64) error {
	if len(unitIDs) > config.MaxUnitsInAntichain {
		unitIDs = unitIDs[:config.MaxUnitsInAntichain]
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(unitIDs)))
	_, err := conn.Write(buf[:4])
	if err != nil {
		return err
	}
	for _, id := range unitIDs {
		binary.LittleEndian.PutUint64(buf, id)
		_, err := conn.Write(buf)
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func receiveRequests(conn network.Connection) ([]uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(conn, buf[:4])
	if err != nil {
		return nil, err
	}
	nReqs := binary.LittleEndian.Uint32(buf[:4])
	if nReqs > config.MaxUnitsInAntichain {
		return nil, errors.New("requests too big")
	}
	result := make([]uint64, nReqs)
	for i := range result {
		_, err := io.ReadFull(conn, buf)
		if err != nil {
			return nil, err
		}
		result[i] = binary.LittleEndian.Uint64(buf)
	}
	return result, nil
}

// getUnits returns as many units with the given IDs as it can.
func getUnits(dag gomel.Dag, unitIDs []uint64) []gomel.Unit {
	result := []gomel.Unit{}
	for _, id := range unitIDs {
		units := dag.GetByID(id)
		result = append(result, units...)
	}
	return result
}
