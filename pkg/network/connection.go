package network

import (
	"time"

	"github.com/rs/zerolog"
)

// Connection represents a connection between two processes.
type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	TimeoutAfter(t time.Duration)
	Log() zerolog.Logger
}
