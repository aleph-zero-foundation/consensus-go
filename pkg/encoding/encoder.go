package encoding

import (
	"encoding/binary"
	"io"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type enc struct {
	io.Writer
}

// newEncoder creates a new encoding.Encoder that is thread-safe.
// It encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 2 bytes.
//  4. Parent heights, as many as declared in 3., 4 bytes each.
//  5. Control hash 32 bytes.
//  6. Size of the unit data in bytes, 4 bytes.
//  7. The unit data, as much as declared in 5.
//  8. Size of the random source data in bytes, 4 bytes.
//  9. The random source data, as much as declared in 7.
// All integer values are encoded as 16 or 32 bit unsigned ints.
func newEncoder(w io.Writer) encoder {
	return &enc{w}
}

// EncodeUnit encodes a unit and writes the encoded data to the io.Writer.
func (e *enc) encodeUnit(unit gomel.Unit) error {
	nParents := uint16(len(unit.Parents()))
	data := make([]byte, 2+64+2+nParents*4+32+4)
	s := 0
	creator := uint16(unit.Creator())
	binary.LittleEndian.PutUint16(data[s:s+2], creator)
	s += 2
	copy(data[s:s+64], unit.Signature())
	s += 64
	binary.LittleEndian.PutUint16(data[s:s+2], nParents)
	s += 2
	for _, p := range unit.Parents() {
		if p != nil {
			binary.LittleEndian.PutUint32(data[s:s+4], uint32(p.Height()))
		} else {
			binary.LittleEndian.PutUint32(data[s:s+4], math.MaxUint32)
		}
		s += 4
	}
	copy(data[s:s+32], unit.ControlHash()[:])
	s += 32

	unitDataLen := uint32(len(unit.Data()))
	binary.LittleEndian.PutUint32(data[s:s+4], unitDataLen)
	s += 4
	_, err := e.Write(data)
	if err != nil {
		return err
	}
	if unitDataLen > 0 {
		_, err = e.Write(unit.Data())
	}
	if err != nil {
		return err
	}

	rsDataLen := uint32(len(unit.RandomSourceData()))
	binary.LittleEndian.PutUint32(data[:4], rsDataLen)
	_, err = e.Write(data[:4])
	if err != nil {
		return err
	}
	if rsDataLen > 0 {
		_, err = e.Write(unit.RandomSourceData())
	}
	if err != nil {
		return err
	}

	return nil
}

func (e *enc) encodeAntichain(units []gomel.Unit) error {
	err := e.encodeUint32(uint32(len(units)))
	if err != nil {
		return err
	}
	for _, u := range units {
		err = e.encodeUnit(u)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *enc) encodeChunk(units []gomel.Unit) error {
	layers := toLayers(units)
	err := e.encodeUint32(uint32(len(layers)))
	if err != nil {
		return err
	}
	for _, layer := range layers {
		err := e.encodeAntichain(layer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *enc) encodeUint32(i uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, i)
	_, err := e.Write(buf)
	return err
}
