package network

import "time"

// Connection represents a connection between two processes.
type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	TimeoutAfter(t time.Duration)
	// PID of the committee member on the other side of the connection.
	Pid() uint16
	// Sync ID, serial number counted for each PID separately.
	Sid() uint32
}
