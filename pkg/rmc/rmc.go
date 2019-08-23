// Package rmc implements a reliable multicast for arbitrary data.
//
// This protocol is based on RBC (reliable broadcast), but has slightly different guarantees.
// Crucially a piece of data multicast with RMC with a given id will agree among all who received it, i.e. it is unique.
// The protocol has no hard guarantees pertaining pessimistic message complexity,
// but can be used in tandem with gossip protocols to disseminate data with proofs of uniqueness.
package rmc

import (
	"errors"
	"io"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/multi"
)

// RMC is a structure holding all data related to a series of reliable multicasts.
type RMC struct {
	inMx, outMx sync.RWMutex
	keys        *multi.Keychain
	in          map[uint64]*incoming
	out         map[uint64]*outgoing
}

// New creates a context for executing instances of the reliable multicast.
func New(pubs []*bn256.VerificationKey, priv *bn256.SecretKey) *RMC {
	return &RMC{
		keys: multi.NewKeychain(pubs, priv),
		in:   map[uint64]*incoming{},
		out:  map[uint64]*outgoing{},
	}
}

// AcceptData reads the id from r, followed by the data and signature of the whole thing.
// It verifies that the id matches the provided one, and that the signature was made by pid.
// It returns the data itself, for protocol-independent verification.
func (rmc *RMC) AcceptData(id uint64, pid uint16, r io.Reader) ([]byte, error) {
	in, err := rmc.newIncomingInstance(id, pid)
	if err != nil {
		return nil, err
	}
	return in.acceptData(r)
}

// SendSignature writes the signature associated with id to w.
// The signature signs the data together with the id.
func (rmc *RMC) SendSignature(id uint64, w io.Writer) error {
	in, err := rmc.getIn(id)
	if err != nil {
		return err
	}
	return in.sendSignature(w)
}

// AcceptProof reads a proof from r and verifies it is a proof that id succeeded.
func (rmc *RMC) AcceptProof(id uint64, r io.Reader) error {
	in, err := rmc.getIn(id)
	if err != nil {
		return err
	}
	return in.acceptProof(r)
}

// SendData writes data catenated with the id and signed by us to w.
func (rmc *RMC) SendData(id uint64, data []byte, w io.Writer) error {
	if rmc.Status(id) != Unknown {
		out, err := rmc.getOut(id)
		if err != nil {
			return err
		}
		return out.sendData(w)
	}
	out, err := rmc.newOutgoingInstance(id, data)
	if err != nil {
		return err
	}
	return out.sendData(w)
}

// AcceptSignature reads a signature from r and verifies it represents pid signing the data associated with id.
// It returns true when at least threshold signatures have been gathered, and the proof has been accumulated.
func (rmc *RMC) AcceptSignature(id uint64, pid uint16, r io.Reader) (bool, error) {
	out, err := rmc.getOut(id)
	if err != nil {
		return false, err
	}
	return out.acceptSignature(pid, r)
}

// SendProof writes the proof associated with id to w.
func (rmc *RMC) SendProof(id uint64, w io.Writer) error {
	out, err := rmc.getOut(id)
	if err != nil {
		return err
	}
	return out.sendProof(w)
}

// SendFinished writes the data and proof associated with id to w.
func (rmc *RMC) SendFinished(id uint64, w io.Writer) error {
	ins, err := rmc.get(id)
	if err != nil {
		return err
	}
	return ins.sendFinished(w)
}

// AcceptFinished reads a pair of data and proof from r and verifies it corresponds to a successfuly finished RMC.
func (rmc *RMC) AcceptFinished(id uint64, pid uint16, r io.Reader) ([]byte, error) {
	in, err := rmc.getIn(id)
	if err != nil {
		in, err = rmc.newIncomingInstance(id, pid)
		if err != nil {
			return nil, err
		}
	}
	return in.acceptFinished(r)
}

// Status returns the state corresponding to id.
func (rmc *RMC) Status(id uint64) Status {
	ins, err := rmc.get(id)
	if err != nil {
		return Unknown
	}
	return ins.status
}

// Data returns the raw data corresponding to id.
// If the status differs from Finished, this data might be unreliable!
func (rmc *RMC) Data(id uint64) []byte {
	ins, err := rmc.get(id)
	if err != nil {
		return nil
	}
	return ins.data()
}

// Clear removes all information concerning id.
// After a clear the state is Unknown until any further calls with id.
func (rmc *RMC) Clear(id uint64) {
	rmc.inMx.Lock()
	defer rmc.inMx.Unlock()
	rmc.outMx.Lock()
	defer rmc.outMx.Unlock()
	delete(rmc.in, id)
	delete(rmc.out, id)
}

func (rmc *RMC) newIncomingInstance(id uint64, pid uint16) (*incoming, error) {
	result := newIncoming(id, pid, rmc.keys)
	rmc.inMx.Lock()
	defer rmc.inMx.Unlock()
	if _, ok := rmc.in[id]; ok {
		return nil, errors.New("duplicate incoming")
	}
	rmc.in[id] = result
	return result, nil
}

func (rmc *RMC) newOutgoingInstance(id uint64, data []byte) (*outgoing, error) {
	result := newOutgoing(id, data, rmc.keys)
	rmc.outMx.Lock()
	defer rmc.outMx.Unlock()
	if _, ok := rmc.out[id]; ok {
		return nil, errors.New("duplicate outgoing")
	}
	rmc.out[id] = result
	return result, nil
}

func (rmc *RMC) getIn(id uint64) (*incoming, error) {
	rmc.inMx.RLock()
	defer rmc.inMx.RUnlock()
	result, ok := rmc.in[id]
	if !ok {
		return nil, errors.New("unknown incoming")
	}
	return result, nil
}

func (rmc *RMC) getOut(id uint64) (*outgoing, error) {
	rmc.outMx.RLock()
	defer rmc.outMx.RUnlock()
	result, ok := rmc.out[id]
	if !ok {
		return nil, errors.New("unknown outgoing")
	}
	return result, nil
}

func (rmc *RMC) get(id uint64) (*instance, error) {
	if in, err := rmc.getIn(id); err == nil {
		return &in.instance, nil
	}
	if out, err := rmc.getOut(id); err == nil {
		return &out.instance, nil
	}
	return nil, errors.New("unknown instance")
}
