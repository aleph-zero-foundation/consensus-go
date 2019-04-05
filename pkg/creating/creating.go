package creating

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

func NewUnit(poset gomel.Poset, creator int) (gomel.Preunit, error) {
	// TODO: implement creation algorithm
	return &preunit{}, nil
}
