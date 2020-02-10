package gomel

import (
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

//shallbedone: should that be here?

// ToPreblock extracts preblock from a given timing round.
// It assumes that
// 1. given slice of units forms a timing round,
// 2. timing unit is the last unit in the slice,
// 3. random source data of the timing unit starts with
// random bytes from the previous level.
func ToPreblock(round []Unit) *core.Preblock {
	data := make([]core.Data, 0, len(round))
	for _, u := range round {
		data = append(data, u.Data())
	}
	randomBytes := round[len(round)-1].RandomSourceData()[:bn256.SignatureLength]
	return core.NewPreblock(data, randomBytes)
}
