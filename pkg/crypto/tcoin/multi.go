package tcoin

import (
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// CreateMulticoin generates a multiCoin for the given ThresholdCoins
// i.e. a ThresholdCoin which corresponds to the sum of polynomials
// which are defining the given ThresholdCoins.
// We assume that:
//  (0) tcs is a non-empty slice
//  (1) the threshold is the same for all given thresholdCoins
//  (2) the thresholdCoins were created by different processes
//  (3) the thresholdCoins have the same owner
//
// The resulting ThresholdCoin has undefined dealer and encSKs.
func CreateMulticoin(tcs []*ThresholdCoin) *ThresholdCoin {
	n := len(tcs[0].vks)
	var result = ThresholdCoin{
		owner:     tcs[0].owner,
		threshold: tcs[0].threshold,
		vks:       make([]*bn256.VerificationKey, n),
	}
	for _, tc := range tcs {
		result.sk = bn256.AddSecretKeys(result.sk, tc.sk)
		result.globalVK = bn256.AddVerificationKeys(result.globalVK, tc.globalVK)
		for i, vk := range tc.vks {
			result.vks[i] = bn256.AddVerificationKeys(result.vks[i], vk)
		}
	}
	return &result
}

// SumShares return a share for a multicoin given shares for
// tcoins forming the multicoin. All the shares should be created by
// the same process.
// The given slice of CoinShares has to be non empty.
func SumShares(cses []*CoinShare) *CoinShare {
	var sum *bn256.Signature
	for _, cs := range cses {
		sum = bn256.AddSignatures(sum, cs.sgn)
	}
	return &CoinShare{
		owner: cses[0].owner,
		sgn:   sum,
	}
}
