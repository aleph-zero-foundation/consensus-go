package tests

import (
	"math/big"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
)

func NewCoin(nProc, pid, seed int) gomel.RandomSource {
	rnd := rand.New(rand.NewSource(int64(seed)))
	threshold := nProc/3 + 1

	shareProviders := make(map[int]bool)
	for i := 0; i < nProc; i++ {
		shareProviders[i] = true
	}

	coeffs := make([]*big.Int, threshold)
	for i := 0; i < threshold; i++ {
		coeffs[i] = big.NewInt(0).Rand(rnd, bn256.Order)
	}

	return coin.New(nProc, pid, tcoin.New(nProc, pid, coeffs), shareProviders)
}
