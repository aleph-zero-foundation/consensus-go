package unit

import (
	"bytes"
	"encoding/binary"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator   uint16
	epochID   gomel.EpochID
	height    int
	signature gomel.Signature
	hash      *gomel.Hash
	crown     *gomel.Crown
	data      core.Data
	rsData    []byte
}

// NewPreunit constructs new preunit from the provided data.
func NewPreunit(id uint64, crown *gomel.Crown, data core.Data, rsData []byte, signature gomel.Signature) gomel.Preunit {
	h, creator, epoch := gomel.DecodeID(id)
	if h != crown.Heights[creator]+1 {
		panic("Inconsistent height information in preunit id and crown")
	}
	pu := &preunit{
		creator:   creator,
		epochID:   epoch,
		height:    h,
		signature: signature,
		crown:     crown,
		data:      data,
		rsData:    rsData,
	}
	pu.hash = ComputeHash(id, crown, data, rsData)
	return pu
}

func (pu *preunit) EpochID() gomel.EpochID {
	return pu.epochID
}

// RandomSourceData embedded in the preunit.
func (pu *preunit) RandomSourceData() []byte {
	return pu.rsData
}

// Data embedded in the preunit.
func (pu *preunit) Data() core.Data {
	return pu.data
}

// Creator of the preunit.
func (pu *preunit) Creator() uint16 {
	return pu.creator
}

// Height of the preunit.
func (pu *preunit) Height() int {
	return pu.height
}

// Signature of the preunit.
func (pu *preunit) Signature() gomel.Signature {
	return pu.signature
}

// Hash of the preunit.
func (pu *preunit) Hash() *gomel.Hash {
	return pu.hash
}

// View returns crown consisting all the parents of the unit.
func (pu *preunit) View() *gomel.Crown {
	return pu.crown
}

// ComputeHash calculates the value of unit's hash based on provided data.
func ComputeHash(id uint64, crown *gomel.Crown, data core.Data, rsData []byte) *gomel.Hash {
	var buf bytes.Buffer
	idBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(idBytes, id)
	buf.Write(idBytes)
	buf.Write(data)
	buf.Write(rsData)
	heightBytes := make([]byte, 4)
	for _, h := range crown.Heights {
		if h == -1 {
			binary.LittleEndian.PutUint32(heightBytes, math.MaxUint32)
		} else {
			binary.LittleEndian.PutUint32(heightBytes, uint32(h))
		}
		buf.Write(heightBytes)
	}
	buf.Write(crown.ControlHash[:])
	result := &gomel.Hash{}
	sha3.ShakeSum128(result[:], buf.Bytes())
	return result
}
