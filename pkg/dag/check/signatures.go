package check

import (
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type signatureChecking struct {
	gomel.Dag
	keys []gomel.PublicKey
}

// Signatures returns a version of the dag that will check signatures.
func Signatures(dag gomel.Dag, keys []gomel.PublicKey) (gomel.Dag, error) {
	if len(keys) != int(dag.NProc()) {
		return nil, errors.New("wrong number of keys provided")
	}
	return &signatureChecking{
		dag, keys,
	}, nil
}

func (dag *signatureChecking) Decode(pu gomel.Preunit) (gomel.Unit, error) {
	if int(pu.Creator()) >= len(dag.keys) {
		return nil, errors.New("invalid creator")
	}
	err := dag.verifySignature(pu)
	if err != nil {
		return nil, err
	}
	return dag.Dag.Decode(pu)
}

func (dag *signatureChecking) verifySignature(pu gomel.Preunit) error {
	if !dag.keys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("invalid signature")
	}
	return nil
}
