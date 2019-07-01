package network

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Multicaster allows to send out messages to multiple recipients
type Multicaster struct {
	conns []Connection
}

// NewMulticaster constructs an instance of Multicaster
func NewMulticaster(conns []Connection) *Multicaster {
	return &Multicaster{
		conns: conns,
	}
}

// Write multicasts bytes b through all conns
func (m *Multicaster) Write(b []byte) (int, error) {
	tasks := make([]func() error, len(m.conns))
	for i, conn := range m.conns {
		i := i
		conn := conn
		tasks[i] = func() error {
			_, err := conn.Write(b)
			return err
		}
	}

	eg := gomel.NewErrGroup()
	if err := eg.Go(tasks); err != nil {
		return 0, err
	}

	return len(b), nil
}

// Flush writes the current state of the internal buffer
func (m *Multicaster) Flush() error {
	tasks := make([]func() error, len(m.conns))
	for i, conn := range m.conns {
		tasks[i] = func() error { return conn.Flush() }
	}

	eg := gomel.NewErrGroup()
	if err := eg.Go(tasks); err != nil {
		return err
	}

	return nil
}

// Close closes all connections
func (m *Multicaster) Close() error {
	for _, conn := range m.conns {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
