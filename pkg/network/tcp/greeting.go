package tcp

import (
	"encoding/binary"
	"errors"
	"io"
)

// greeting represents data sent at the beginning of a sync to identify oneself.
type greeting struct {
	pid uint32
	sid uint32
}

// MarshalBinary encodes the greeting as a slice of bytes.
func (g *greeting) MarshalBinary() ([]byte, error) {
	var result [8]byte
	binary.LittleEndian.PutUint32(result[0:], g.pid)
	binary.LittleEndian.PutUint32(result[4:], g.sid)
	return result[:], nil
}

// UnmarshalBinary decodes the greeting encoded as a slice of bytes.
func (g *greeting) UnmarshalBinary(data []byte) error {
	if len(data) != 8 {
		return errors.New("bad greeting data")
	}
	g.pid = binary.LittleEndian.Uint32(data[0:])
	g.sid = binary.LittleEndian.Uint32(data[4:])
	return nil
}

// send writes the greeting as bytes to theprovided writer.
func (g *greeting) send(there io.Writer) error {
	data, err := g.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = there.Write(data)
	return err
}

// getGreeting reads from the provided source and interprets the data received as a greeting.
func getGreeting(there io.Reader) (*greeting, error) {
	var result greeting
	var data [8]byte
	alreadyRead := 0
	for alreadyRead < len(data) {
		read, err := there.Read(data[alreadyRead:])
		alreadyRead += read
		if err != nil {
			return nil, err
		}
	}
	result.UnmarshalBinary(data[:])
	return &result, nil
}
