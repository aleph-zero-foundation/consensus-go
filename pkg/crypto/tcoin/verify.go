package tcoin

import (
	"crypto/subtle"
	"encoding/binary"
	"math/big"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
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

// VerifyWrongSecretKeyProof verifies the proof given by a process that
// his secretKey is incorrect.
// We check whether:
// (1) Enc_prover(dealer, proof) = EncSK[prover]
// (2) vk[prover] is not a verification key for the proof
func (tc *ThresholdCoin) VerifyWrongSecretKeyProof(prover uint16, proof *bn256.SecretKey, encryptionKey encrypt.EncryptionKey) bool {

	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, tc.dealer)
	encSK, err := encryptionKey.Encrypt(append(buf, proof.Marshal()...))
	if err != nil {
		return false
	}

	if !encrypt.CTEq(tc.encSKs[prover], encSK) {
		return false
	}

	vk := proof.VerificationKey()
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[prover].Marshal()) != 1 {
		return true
	}
	return false
}
