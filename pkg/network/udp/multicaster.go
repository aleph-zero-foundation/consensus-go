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
	//might be a good idea to execute this loop in parallel?
	//also, it deserves better error handling
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
