// Package custom implements custom encoding of units.
//
// Note that the objects being encoded are units, while the objects resulting from decoding are preunits.
// This makes perfect sense, as we only want to send information about units we already added to our dag,
// and any decoded information will have to be manually included into the dag.
//
// Crucially, and in contrast to Gob, this encoding only reads the bytes it needs.
package custom

import (
	"encoding/binary"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type encoder struct {
	writer io.Writer
}

// NewEncoder creates a new encoding.Encoder that is threadsafe.
// It encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 2 bytes.
//  4. Parent hashes, as many as declared in 3., 32 bytes each.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
//  7. Size of the random source data in bytes, 4 bytes.
//  8. The random source data, as much as declared in 7.
// All integer values are encoded as 16 or 32 bit unsigned ints.
func NewEncoder(w io.Writer) encoding.Encoder {
	return &encoder{w}
}

// EncodeUnit encodes a unit and writes the encoded data to the io.Writer.
func (e *encoder) EncodeUnit(unit gomel.Unit) error {
	nParents := uint16(len(unit.Parents()))
	data := make([]byte, 2+64+2+nParents*32+4)
	s := 0
	creator := uint16(unit.Creator())
	binary.LittleEndian.PutUint16(data[s:s+2], creator)
	s += 2
	copy(data[s:s+64], unit.Signature())
	s += 64
	binary.LittleEndian.PutUint16(data[s:s+2], nParents)
	s += 2
	for _, p := range unit.Parents() {
		copy(data[s:s+32], p.Hash()[:])
		s += 32
	}

	unitDataLen := uint32(len(unit.Data()))
	binary.LittleEndian.PutUint32(data[s:s+4], unitDataLen)
	s += 4
	_, err := e.writer.Write(data)
	if err != nil {
		return err
	}
	if unitDataLen > 0 {
		_, err = e.writer.Write(unit.Data())
	}
	if err != nil {
		return err
	}

	rsDataLen := uint32(len(unit.RandomSourceData()))
	binary.LittleEndian.PutUint32(data[:4], rsDataLen)
	s += 4
	_, err = e.writer.Write(data[:4])
	if err != nil {
		return err
	}
	if rsDataLen > 0 {
		_, err = e.writer.Write(unit.RandomSourceData())
	}
	if err != nil {
		return err
	}

	return nil
}

type decoder struct {
	reader io.Reader
}

// NewDecoder creates a new encoding.Decoder that is threadsafe.
// It assumes the data encodes units in the following format:
//  1. Creator id, 2 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 2 bytes.
//  4. Parent hashes, as many as declared in 3., 32 bytes each.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
//  7. Size of the random source data in bytes, 4 bytes.
//  8. The random source data, as much as declared in 7.
// All integer values are encoded as 16 or 32 bit unsigned ints.
// It is guaranteed to read only as much data as needed.
func NewDecoder(r io.Reader) encoding.Decoder {
	return &decoder{r}
}

// DecodePreunit reads encoded data from the io.Reader and tries to decode it
// as a preunit.
func (d *decoder) DecodePreunit() (gomel.Preunit, error) {
	uint16Buf := make([]byte, 2)
	uint32Buf := make([]byte, 4)
	_, err := io.ReadFull(d.reader, uint16Buf)
	if err != nil {
		return nil, err
	}
	creator := binary.LittleEndian.Uint16(uint16Buf)
	signature := make([]byte, 64)
	_, err = io.ReadFull(d.reader, signature)
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(d.reader, uint16Buf)
	if err != nil {
		return nil, err
	}
	nParents := binary.LittleEndian.Uint16(uint16Buf)
	parents := make([]*gomel.Hash, nParents)
	for i := range parents {
		parents[i] = &gomel.Hash{}
		_, err = io.ReadFull(d.reader, parents[i][:])
		if err != nil {
			return nil, err
		}
	}
	_, err = io.ReadFull(d.reader, uint32Buf)
	if err != nil {
		return nil, err
	}
	unitDataLen := binary.LittleEndian.Uint32(uint32Buf)
	unitData := make([]byte, unitDataLen)
	_, err = io.ReadFull(d.reader, unitData)
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(d.reader, uint32Buf)
	if err != nil {
		return nil, err
	}
	rsDataLen := binary.LittleEndian.Uint32(uint32Buf)
	rsData := make([]byte, rsDataLen)
	_, err = io.ReadFull(d.reader, rsData)
	if err != nil {
		return nil, err
	}

	result := creating.NewPreunit(int(creator), parents, unitData, rsData)
	result.SetSignature(signature)
	return result, nil
}
