package gomel

import (
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// ToPreblock produces a preblock from a slice of units containing a timing round.
// It assumes that the timing unit is the last unit in the slice, and that random
// source data of the timing unit starts with random bytes from the previous level.
func ToPreblock(round []Unit) *core.Preblock {
	data := make([]core.Data, 0, len(round))
	for _, u := range round {
		data = append(data, u.Data())
	}
	randomBytes := round[len(round)-1].RandomSourceData()[:bn256.SignatureLength]
	return core.NewPreblock(data, randomBytes)
}
