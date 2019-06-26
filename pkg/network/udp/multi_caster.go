package udp

import (
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type multicaster struct {
	conns []network.Connection
}

func newMulticaster(conns []network.Connection) *multicaster {
	return &multicaster{
		conns: conns,
	}
}

func (m *multicaster) Write(b []byte) (int, error) {
	for _, conn := range m.conns {
		_, err := conn.Write(b)
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func (m *multicaster) Flush() error {
	for _, conn := range m.conns {
		err := conn.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *multicaster) Close() error {
	for _, conn := range m.conns {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
