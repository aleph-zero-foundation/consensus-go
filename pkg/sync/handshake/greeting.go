// Package handshake implements protocols for identifying the peer.
//
// These protocols are used before some proper sync protocols, to figure out who we are talking to.
// This is currently not cryptographically secure, but as long as we don't punish peers,
// which send some wrong information this shouldn't impact anything negatively.
package handshake

import (
	"encoding/binary"
	"io"
)

// Greet sends a greeting to the given conn.
func Greet(w io.Writer, pid uint16, sid uint32) error {
	var data [6]byte
	binary.LittleEndian.PutUint16(data[0:], pid)
	binary.LittleEndian.PutUint32(data[2:], sid)
	_, err := w.Write(data[:])
	return err
}

// AcceptGreeting accepts a greeting and returns the information it learned from it.
func AcceptGreeting(r io.Reader) (pid uint16, sid uint32, err error) {
	var data [6]byte
	_, err = io.ReadFull(r, data[:])
	if err != nil {
		return
	}
	pid = binary.LittleEndian.Uint16(data[0:])
	sid = binary.LittleEndian.Uint32(data[2:])
	return
}
