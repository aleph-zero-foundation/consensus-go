package beacon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

// TODO: This could be encoded in a more optimal way.
// For exmaple we could have use a bitset
// We start with very simple but inefficient implementation
//
// Votes are encoded one by one. Byte representation of a vote is
// (1) vote type (1 byte), 0 - nil, 1 - yes, 2 - no
// if the vote type is 2
// (2) length of proof (2 bytes)
// (3) proof itself encoed in Gob (as much as declared in 2)
func marshallVotes(votes []*vote) []byte {
	var buf bytes.Buffer
	for _, v := range votes {
		if v == nil {
			buf.Write([]byte{0})
		} else if v.isCorrect == true {
			buf.Write([]byte{1})
		} else {
			buf.Write([]byte{2})
			proofBytes, _ := v.proof.GobEncode()
			binary.Write(&buf, binary.LittleEndian, uint16(len(proofBytes)))
			buf.Write(proofBytes)
		}
	}
	return buf.Bytes()
}

func unmarshallVotes(data []byte, nProc int) ([]*vote, error) {
	votes := make([]*vote, nProc)
	for pid := 0; pid < nProc; pid++ {
		if len(data) < 1 {
			return nil, errors.New("votes wrongly encoded")
		}
		if data[0] == 0 {
			votes[pid] = nil
			data = data[1:]
		} else if data[0] == 1 {
			data = data[1:]
			votes[pid] = &vote{
				isCorrect: true,
				proof:     nil,
			}
		} else {
			data = data[1:]
			if len(data) < 2 {
				return nil, errors.New("votes wrongly encoded")
			}
			proofLen := binary.LittleEndian.Uint16(data[:2])
			data = data[2:]
			if len(data) < int(proofLen) {
				return nil, errors.New("votes wrongly encoded")
			}
			proof := big.NewInt(0)
			err := proof.GobDecode(data[:proofLen])
			data = data[proofLen:]
			if err != nil {
				return nil, err
			}
			votes[pid] = &vote{
				isCorrect: false,
				proof:     proof,
			}
		}
	}
	return votes, nil
}

// Shares are encoded in the following way:
// (1) 0 or 1 (1 byte) indicating if the shrae is nil
// if the share is not nil
// (2) length of the marshalled share (2 bytes)
// (3) marshalled share (as much as declared in 2)
func marshallShares(cses []*tcoin.CoinShare) []byte {
	var buf bytes.Buffer
	for _, cs := range cses {
		if cs == nil {
			buf.Write([]byte{0})
		} else {
			buf.Write([]byte{1})
			csMarshalled := cs.Marshal()
			binary.Write(&buf, binary.LittleEndian, uint16(len(csMarshalled)))
			buf.Write(csMarshalled)
		}
	}
	return buf.Bytes()
}

func unmarshallShares(data []byte, nProc int) ([]*tcoin.CoinShare, error) {
	shares := make([]*tcoin.CoinShare, nProc)
	for pid := 0; pid < nProc; pid++ {
		if len(data) < 1 {
			return nil, errors.New("cses wrongly encoded")
		}
		if data[0] == 0 {
			shares[pid] = nil
			data = data[1:]
		} else if data[0] == 1 {
			data = data[1:]
			if len(data) < 2 {
				return nil, errors.New("cses wrongly encoded")
			}
			csLen := binary.LittleEndian.Uint16(data[:2])
			data = data[2:]
			if len(data) < int(csLen) {
				return nil, errors.New("cses wrongly encoded")
			}
			cs := new(tcoin.CoinShare)
			err := cs.Unmarshal(data[:csLen])
			if err != nil {
				return nil, err
			}
			data = data[csLen:]
			shares[pid] = cs
		}
	}
	return shares, nil
}
