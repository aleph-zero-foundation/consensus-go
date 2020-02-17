package fetch

import (
	"encoding/binary"
	"errors"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/core-go/pkg/network"
)

// Request is a query for fetch server to perform a sync with the given process and request particular units.
type Request struct {
	Pid     uint16
	UnitIDs []uint64
}

func sendRequests(conn network.Connection, unitIDs []uint64) error {
	if len(unitIDs) > config.MaxUnitsInChunk {
		unitIDs = unitIDs[:config.MaxUnitsInChunk]
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
	if nReqs > config.MaxUnitsInChunk {
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
