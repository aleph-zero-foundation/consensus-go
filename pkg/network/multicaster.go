package network

// Multicaster allows to send out messages to multiple recipients
type Multicaster struct {
	conns []Connection
}

func NewMulticaster(conns []Connection) *Multicaster {
	return &Multicaster{
		conns: conns,
	}
}

func (m *Multicaster) Write(b []byte) (int, error) {
	//might be a good idea to execute this loop in parallel?
	//also, it deserves better error handling
	for _, conn := range m.conns {
		_, err := conn.Write(b)
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func (m *Multicaster) Flush() error {
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

func (m *Multicaster) Close() error {
	for _, conn := range m.conns {
		err := conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
