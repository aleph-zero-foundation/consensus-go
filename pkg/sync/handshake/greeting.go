package handshake

import (
	"encoding/binary"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Greet sends a greeting to the given conn.
func Greet(conn network.Connection, pid uint16, sid uint32) error {
	var data [6]byte
	binary.LittleEndian.PutUint16(data[0:], pid)
	binary.LittleEndian.PutUint32(data[2:], sid)
	_, err := conn.Write(data[:])
	if err != nil {
		return err
	}
	return conn.Flush()
}

// AcceptGreeting accepts a greeting and returns the information it learned from it.
func AcceptGreeting(conn network.Connection) (pid uint16, sid uint32, err error) {
	var data [6]byte
	_, err = io.ReadFull(conn, data[:])
	if err != nil {
		return
	}
	pid = binary.LittleEndian.Uint16(data[0:])
	sid = binary.LittleEndian.Uint32(data[2:])
	return
}
