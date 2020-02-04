package tcoin

import (
	"crypto/subtle"
	"math/big"

	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
)

// VerifyCoinShare verifies whether the given coin share is correct.
func (tc *ThresholdCoin) VerifyCoinShare(share *CoinShare, nonce int) bool {
	return tc.vks[share.owner].Verify(share.sgn, big.NewInt(int64(nonce)).Bytes())
}

// VerifyCoin verifies whether the given coin is correct.
func (tc *ThresholdCoin) VerifyCoin(c *Coin, nonce int) bool {
	return tc.globalVK.Verify(c.sgn, big.NewInt(int64(nonce)).Bytes())
}

// PolyVerify uses the given polyVerifier to verify if the vks form
// a polynomial sequence.
func (tc *ThresholdCoin) PolyVerify(pv bn256.PolyVerifier) bool {
	return pv.Verify(tc.vks)
}

// VerifySecretKey checks if the verificationKey and secretKey form a valid pair.
// It returns the incorrect secret key when the pair of keys is invalid or
// nil when the keys are valid.
func (tc *ThresholdCoin) VerifySecretKey() *bn256.SecretKey {
	vk := tc.sk.VerificationKey()
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[tc.owner].Marshal()) != 1 {
		return tc.sk
	}
	return nil
}
