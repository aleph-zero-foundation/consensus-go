package rmc

import (
	"errors"
	"io"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/multi"
)

// Protocol is a structure holding all data related to a series of reliable multicasts.
type Protocol struct {
	sync.RWMutex
	keys *multi.Keychain
	in   map[uint64]*incoming
	out  map[uint64]*outgoing
}

// New creates an context for executing instances of the reliable multicast.
func New(pubs []*bn256.VerificationKey, priv *bn256.SecretKey) *Protocol {
	return &Protocol{
		keys: multi.NewKeychain(pubs, priv),
		in:   map[uint64]*incoming{},
		out:  map[uint64]*outgoing{},
	}
}

// AcceptData reads data from r, followed by the id and signature.
// It verifies that the id matches the provided one, and that the signature was made by pid.
// It returns the data itself, for protocol-independent verification.
func (p *Protocol) AcceptData(id uint64, pid uint16, r io.Reader) ([]byte, error) {
	in, err := p.newIncomingInstance(id, pid)
	if err != nil {
		return nil, err
	}
	return in.acceptData(r)
}

// SendSignature writes the signature associated with id to w.
// The signature signs the data together with the id.
func (p *Protocol) SendSignature(id uint64, w io.Writer) error {
	in, err := p.getIn(id)
	if err != nil {
		return err
	}
	return in.sendSignature(w)
}

// AcceptProof reads a proof from r and verifies it is a proof that id succeeded.
func (p *Protocol) AcceptProof(id uint64, r io.Reader) error {
	in, err := p.getIn(id)
	if err != nil {
		return err
	}
	return in.acceptProof(r)
}

// SendData writes data catenated with the id and signed by us to w.
func (p *Protocol) SendData(id uint64, data []byte, w io.Writer) error {
	if p.Status(id) != Unknown {
		out, err := p.getOut(id)
		if err != nil {
			return err
		}
		return out.sendData(w)
	}
	out, err := p.newOutgoingInstance(id, data)
	if err != nil {
		return err
	}
	return out.sendData(w)
}

// AcceptSignature reads a signature from r and verifies it represents pid signing the data associated with id.
// It returns true when at least threshold signatures have been gathered, and the proof has been accumulated.
func (p *Protocol) AcceptSignature(id uint64, pid uint16, r io.Reader) (bool, error) {
	out, err := p.getOut(id)
	if err != nil {
		return false, err
	}
	return out.acceptSignature(pid, r)
}

// SendProof writes the proof associated with id to w.
func (p *Protocol) SendProof(id uint64, w io.Writer) error {
	out, err := p.getOut(id)
	if err != nil {
		return err
	}
	return out.sendProof(w)
}

// SendFinished writes the data and proof associated with id to w.
func (p *Protocol) SendFinished(id uint64, w io.Writer) error {
	ins, err := p.get(id)
	if err != nil {
		return err
	}
	return ins.sendFinished(w)
}

// AcceptFinished reads a pair of data and proof from r and verifies it corresponds to a successfuly finished RMC.
func (p *Protocol) AcceptFinished(id uint64, pid uint16, r io.Reader) ([]byte, error) {
	in, err := p.getIn(id)
	if err != nil {
		in, err = p.newIncomingInstance(id, pid)
		if err != nil {
			return nil, err
		}
	}
	return in.acceptFinished(r)
}

// Status returns the state corresponding to id.
func (p *Protocol) Status(id uint64) Status {
	ins, err := p.get(id)
	if err != nil {
		return Unknown
	}
	return ins.status
}

// Data returns the raw data corresponding to id.
// If the status differs from Finished, this data might be unreliable!
func (p *Protocol) Data(id uint64) []byte {
	ins, err := p.get(id)
	if err != nil {
		return nil
	}
	return ins.data()
}

// Clear removes all information concerning id.
// After a clear the state is Unknown until any further calls with id.
func (p *Protocol) Clear(id uint64) {
	p.Lock()
	defer p.Unlock()
	delete(p.in, id)
	delete(p.out, id)
}

func (p *Protocol) newIncomingInstance(id uint64, pid uint16) (*incoming, error) {
	result := newIncoming(id, pid, p.keys)
	p.Lock()
	defer p.Unlock()
	if _, ok := p.in[id]; ok {
		return nil, errors.New("duplicate incoming")
	}
	p.in[id] = result
	return result, nil
}

func (p *Protocol) newOutgoingInstance(id uint64, data []byte) (*outgoing, error) {
	result := newOutgoing(id, data, p.keys)
	p.Lock()
	defer p.Unlock()
	if _, ok := p.out[id]; ok {
		return nil, errors.New("duplicate outgoing")
	}
	p.out[id] = result
	return result, nil
}

func (p *Protocol) getIn(id uint64) (*incoming, error) {
	p.RLock()
	defer p.RUnlock()
	result, ok := p.in[id]
	if !ok {
		return nil, errors.New("unknown incoming")
	}
	return result, nil
}

func (p *Protocol) getOut(id uint64) (*outgoing, error) {
	p.RLock()
	defer p.RUnlock()
	result, ok := p.out[id]
	if !ok {
		return nil, errors.New("unknown outgoing")
	}
	return result, nil
}

func (p *Protocol) get(id uint64) (*instance, error) {
	if in, err := p.getIn(id); err == nil {
		return &in.instance, nil
	}
	if out, err := p.getOut(id); err == nil {
		return &out.instance, nil
	}
	return nil, errors.New("unknown instance")
}
