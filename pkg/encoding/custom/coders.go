package custom

import (
	"encoding/binary"
	"io"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
)

type encoder struct {
	writer io.Writer
}

// NewEncoder creates a new encoding.Encoder that is threadsafe.
// It encodes units in the following format:
//  1. Creator id, 4 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 4 bytes.
//  4. Parent hashes, as many as declared in 3., 64 bytes each.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
// All integer values are encoded as 32 bit unsigned ints.
func NewEncoder(w io.Writer) encoding.Encoder {
	return &encoder{w}
}

// EncodeUnit encodes a unit and writes the encoded data to the io.Writer.
func (e *encoder) EncodeUnit(unit gomel.Unit) error {
	nParents := uint32(len(unit.Parents()))
	data := make([]byte, 64+4+4+nParents*64+4)
	s := 0
	creator := uint32(unit.Creator())
	binary.LittleEndian.PutUint32(data[s:s+4], creator)
	s += 4
	copy(data[s:s+64], unit.Signature())
	s += 64
	binary.LittleEndian.PutUint32(data[s:s+4], nParents)
	s += 4
	for _, p := range unit.Parents() {
		copy(data[s:s+64], p.Hash()[:])
		s += 64
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
	return err
}

type decoder struct {
	reader io.Reader
}

// NewDecoder creates a new encoding.Decoder that is threadsafe.
// It assumes the data encodes units in the following format:
//  1. Creator id, 4 bytes.
//  2. Signature, 64 bytes.
//  3. Number of parents, 4 bytes.
//  4. Parent hashes, as many as declared in 3., 64 bytes each.
//  5. Size of the unit data in bytes, 4 bytes.
//  6. The unit data, as much as declared in 5.
// All integer values are encoded as 32 bit unsigned ints.
// It is guaranteed to read only as much data as needed.
func NewDecoder(r io.Reader) encoding.Decoder {
	return &decoder{r}
}

// DecodePreunit reads encoded data from the io.Reader and tries to decode it
// as a preunit.
func (d *decoder) DecodePreunit() (gomel.Preunit, error) {
	lenData := make([]byte, 4)
	_, err := io.ReadFull(d.reader, lenData)
	if err != nil {
		return nil, err
	}
	creator := binary.LittleEndian.Uint32(lenData)
	signature := make([]byte, 64)
	_, err = io.ReadFull(d.reader, signature)
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(d.reader, lenData)
	if err != nil {
		return nil, err
	}
	nParents := binary.LittleEndian.Uint32(lenData)
	parents := make([]gomel.Hash, nParents)
	for i := range parents {
		_, err = io.ReadFull(d.reader, parents[i][:])
		if err != nil {
			return nil, err
		}
	}
	_, err = io.ReadFull(d.reader, lenData)
	if err != nil {
		return nil, err
	}
	unitDataLen := binary.LittleEndian.Uint32(lenData)
	unitData := make([]byte, unitDataLen)
	_, err = io.ReadFull(d.reader, unitData)
	if err != nil {
		return nil, err
	}
	result := creating.NewPreunit(int(creator), parents, unitData)
	result.SetSignature(signature)
	return result, nil
}
