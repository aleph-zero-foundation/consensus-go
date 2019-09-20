package fetch

import (
	"encoding/binary"
	"fmt"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type request struct {
	pid    uint16
	hashes []*gomel.Hash
}

func sendRequests(conn network.Connection, hashes []*gomel.Hash) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(hashes)))
	_, err := conn.Write(buf)
	if err != nil {
		return err
	}
	for _, h := range hashes {
		_, err = conn.Write(h[:])
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func receiveRequests(conn network.Connection) ([]*gomel.Hash, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	result := make([]*gomel.Hash, binary.LittleEndian.Uint32(buf))
	for i := range result {
		result[i] = &gomel.Hash{}
		_, err = io.ReadFull(conn, result[i][:])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func getUnits(dag gomel.Dag, hashes []*gomel.Hash) ([]gomel.Unit, error) {
	units := dag.Get(hashes)
	for i, u := range units {
		if u == nil {
			return nil, fmt.Errorf("received request for unknown hash: %s", hashes[i].Short()) //TODO pedantic
		}
	}
	return units, nil
}
