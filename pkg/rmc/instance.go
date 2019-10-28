package rmc

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/multi"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type instance struct {
	sync.Mutex
	id         uint64
	keys       *multi.Keychain
	rawLen     uint32
	signedData []byte
	proof      *multi.Signature
	stat       Status
}

func (ins *instance) sendData(w io.Writer) error {
	ins.Lock()
	defer ins.Unlock()
	err := encodeUint32(w, ins.rawLen)
	if err != nil {
		return err
	}
	_, err = w.Write(ins.signedData)
	return err
}

func (ins *instance) sendProof(w io.Writer) error {
	ins.Lock()
	defer ins.Unlock()
	if ins.stat != Finished {
		return errors.New("no proof to send")
	}
	_, err := w.Write(ins.proof.Marshal())
	return err
}

func (ins *instance) sendFinished(w io.Writer) error {
	err := ins.sendData(w)
	if err != nil {
		return err
	}
	return ins.sendProof(w)
}

func (ins *instance) data() []byte {
	return ins.signedData[8 : 8+ins.rawLen]
}

func (ins *instance) status() Status {
	ins.Lock()
	defer ins.Unlock()
	return ins.stat
}

type incoming struct {
	instance
	pid uint16
}

func newIncoming(id uint64, pid uint16, keys *multi.Keychain) *incoming {
	return &incoming{
		instance{
			id:   id,
			keys: keys,
		},
		pid,
	}
}

func (in *incoming) acceptData(r io.Reader) ([]byte, error) {
	rawLen, err := decodeUint32(r)
	if err != nil {
		return nil, err
	}
	signedData := make([]byte, 8+rawLen+multi.SignatureLength)
	_, err = io.ReadFull(r, signedData)
	if err != nil {
		return nil, err
	}
	id := binary.LittleEndian.Uint64(signedData[:8])
	if id != in.id {
		return nil, errors.New("incoming id mismatch")
	}
	if !in.keys.Verify(in.pid, signedData) {
		return nil, errors.New("wrong data signature")
	}
	nProc := uint16(in.keys.Length())
	proof := multi.NewSignature(gomel.MinimalQuorum(nProc), signedData)
	in.Lock()
	defer in.Unlock()
	in.signedData = signedData
	in.rawLen = rawLen
	in.proof = proof
	in.stat = Data
	return in.data(), nil
}

func (in *incoming) sendSignature(w io.Writer) error {
	in.Lock()
	defer in.Unlock()
	if in.stat == Unknown {
		return errors.New("cannot sign unknown data")
	}
	signature := in.keys.Sign(in.signedData)
	_, err := w.Write(signature)
	if err != nil {
		return err
	}
	in.stat = Signed
	return nil
}

func (in *incoming) acceptProof(r io.Reader) error {
	in.Lock()
	defer in.Unlock()
	if in.stat == Unknown {
		return errors.New("cannot accept proof of unknown data")
	}
	data := make([]byte, in.proof.MarshaledLength())
	_, err := io.ReadFull(r, data)
	if err != nil {
		return err
	}
	_, err = in.proof.Unmarshal(data)
	if err != nil {
		return err
	}
	if !in.keys.MultiVerify(in.proof) {
		return errors.New("wrong multisignature")
	}
	in.stat = Finished
	return nil
}

func (in *incoming) acceptFinished(r io.Reader) ([]byte, error) {
	result, err := in.acceptData(r)
	if err != nil {
		return nil, err
	}
	return result, in.acceptProof(r)
}

type outgoing struct {
	instance
}

func newOutgoing(id uint64, data []byte, keys *multi.Keychain) *outgoing {
	rawLen := uint32(len(data))
	buf := make([]byte, 8, 8+rawLen)
	binary.LittleEndian.PutUint64(buf, id)
	buf = append(buf[:8], data...)
	signedData := append(buf, keys.Sign(buf)...)
	nProc := uint16(keys.Length())
	proof := multi.NewSignature(gomel.MinimalQuorum(nProc), signedData)
	proof.Aggregate(keys.Pid(), keys.Sign(signedData))
	return &outgoing{
		instance{
			id:         id,
			keys:       keys,
			rawLen:     rawLen,
			signedData: signedData,
			proof:      proof,
			stat:       Data,
		},
	}
}

func (out *outgoing) acceptSignature(pid uint16, r io.Reader) (bool, error) {
	signature := make([]byte, multi.SignatureLength)
	_, err := io.ReadFull(r, signature)
	out.Lock()
	defer out.Unlock()
	if err != nil {
		return false, err
	}
	if !out.keys.Verify(pid, append(out.signedData, signature...)) {
		return false, errors.New("wrong signature")
	}
	if out.stat != Finished {
		done, err := out.proof.Aggregate(pid, signature)
		if done {
			out.stat = Finished
			return true, err
		}
		return false, err
	}
	return false, nil
}

func encodeUint32(w io.Writer, i uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, i)
	_, err := w.Write(buf)
	return err
}

func decodeUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}
