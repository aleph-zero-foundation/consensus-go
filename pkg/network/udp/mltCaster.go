package udp

import (
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type mltCaster struct {
	conns []network.Connection
}

func newMltCaster(conns []network.Connection) *mltCaster {
	return &mltCaster{
		conns: conns,
	}
}

func (m *mltCaster) Write(b []byte) (int, error) {
	for _, conn := range m.conns {
		_, err := conn.Write(b)
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func (m *mltCaster) Flush() error {
	for _, conn := range m.conns {
		err := conn.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *mltCaster) Close() error {
	for _, conn := range m.conns {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
