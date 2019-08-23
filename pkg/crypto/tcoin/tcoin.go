// Package tcoin implements a threshold coin for generating sequences of random bytes.
//
// This is the main component of all the random sources we use in the algorithm.
package tcoin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
)

// globalThresholdCoin is a threshold coin generated by a dealer.
// It contains secret keys of all processes.
// In the full version of the protocol these secretKeys should be sealed.
type globalThresholdCoin struct {
	threshold int
	globalVK  *bn256.VerificationKey
	vks       []*bn256.VerificationKey
	sks       []*bn256.SecretKey
}

// ThresholdCoin is a threshold coin owned by one of the parties.
type ThresholdCoin struct {
	Threshold int
	pid       int
	globalVK  *bn256.VerificationKey
	vks       []*bn256.VerificationKey
	sk        *bn256.SecretKey
	sks       []*bn256.SecretKey
}

// New returns a ThresholdCoin based on given slice of coefficients.
func New(nProc, pid int, coeffs []*big.Int) *ThresholdCoin {
	threshold := len(coeffs)
	secret := coeffs[threshold-1]

	globalVK := bn256.NewVerificationKey(secret)

	var wg sync.WaitGroup
	var sks = make([]*bn256.SecretKey, nProc)
	var vks = make([]*bn256.VerificationKey, nProc)

	for i := 0; i < nProc; i++ {
		wg.Add(1)
		go func(ind int) {
			defer wg.Done()
			secret := poly(coeffs, big.NewInt(int64(ind+1)))
			sks[ind] = bn256.NewSecretKey(secret)
			vks[ind] = bn256.NewVerificationKey(secret)
		}(i)
	}
	wg.Wait()

	return &ThresholdCoin{
		Threshold: threshold,
		pid:       pid,
		globalVK:  globalVK,
		vks:       vks,
		sks:       sks,
		sk:        sks[pid],
	}
}

// Deal returns byte representation of a threshold coin
// with the given threshold and number of processes.
func Deal(nProcesses, threshold int) []byte {
	return generateThresholdCoin(nProcesses, threshold).encode()
}

// encode returns byte representation of the given gtc in the following form
// (1) threshold, 4 bytes as uint32
// (2) length of marshalled globalVK, 4 bytes as uint32
// (3) marshalled globalVK
// (4) len(vks), 4 bytes as uint32
// (5) Marshalled vks in the form
//     a) length of marshalled vk
//     b) marshalled vk
// (6) Marshalled sks in the form
//     a) length of marshalled sk
//     b) marshalled sk
func (gtc *globalThresholdCoin) encode() []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[:4], uint32(gtc.threshold))

	globalVKMarshalled := gtc.globalVK.Marshal()
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(globalVKMarshalled)))
	data = append(data, globalVKMarshalled...)

	dataLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataLen[:], uint32(len(gtc.vks)))
	data = append(data, dataLen...)

	for _, vk := range gtc.vks {
		vkMarshalled := vk.Marshal()
		binary.LittleEndian.PutUint32(dataLen, uint32(len(vkMarshalled)))
		data = append(data, dataLen...)
		data = append(data, vkMarshalled...)
	}

	for _, sk := range gtc.sks {
		skMarshalled := sk.Marshal()
		binary.LittleEndian.PutUint32(dataLen, uint32(len(skMarshalled)))
		data = append(data, dataLen...)
		data = append(data, skMarshalled...)
	}
	return data
}

// Decode creates tc for given pid from byte representation of gtc.
func Decode(data []byte, pid int) (*ThresholdCoin, error) {
	ind := 0
	dataTooShort := errors.New("Decoding tcoin failed. Given bytes slice is too short")
	if len(data) < ind+4 {
		return nil, dataTooShort
	}
	threshold := int(binary.LittleEndian.Uint32(data[:(ind + 4)]))
	ind += 4

	if len(data) < ind+4 {
		return nil, dataTooShort
	}
	gvkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	if len(data) < ind+gvkLen {
		return nil, dataTooShort
	}
	globalVK, err := new(bn256.VerificationKey).Unmarshal(data[ind:(ind + gvkLen)])
	if err != nil {
		return nil, errors.New("unmarshal of globalVK failed")
	}
	ind += gvkLen

	if len(data) < ind+4 {
		return nil, dataTooShort
	}
	nProcesses := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	vks := make([]*bn256.VerificationKey, nProcesses)
	for i := range vks {
		if len(data) < ind+4 {
			return nil, dataTooShort
		}
		vkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+vkLen {
			return nil, dataTooShort
		}
		vks[i], err = new(bn256.VerificationKey).Unmarshal(data[ind:(ind + vkLen)])
		if err != nil {
			return nil, errors.New("unmarshal of vk failed")
		}
		ind += vkLen
	}
	sks := make([]*bn256.SecretKey, nProcesses)
	for i := range sks {
		if len(data) < ind+4 {
			return nil, dataTooShort
		}
		skLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		if len(data) < ind+skLen {
			return nil, dataTooShort
		}
		sks[i], err = new(bn256.SecretKey).Unmarshal(data[ind:(ind + skLen)])
		if err != nil {
			return nil, errors.New("unmarshal of sk failed")
		}
		ind += skLen
	}
	return &ThresholdCoin{
		Threshold: threshold,
		globalVK:  globalVK,
		vks:       vks,
		sk:        sks[pid],
		sks:       sks,
		pid:       pid,
	}, nil
}

// CreateMulticoin generates a multiCoin for the given ThresholdCoins
// i.e. a ThresholdCoin which corresponds to the sum of polynomials
// which are defining the given ThresholdCoins.
// We assume that:
//  (0) tcs is a non-empty slice
//  (1) the threshold is the same for all given thresholdCoins
//  (2) the thresholdCoins were created by different processes
//  (3) the thresholdCoins have the same owner
func CreateMulticoin(tcs []*ThresholdCoin) *ThresholdCoin {
	n := len(tcs[0].vks)
	var result = ThresholdCoin{
		Threshold: tcs[0].Threshold,
		pid:       tcs[0].pid,
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

// generateThresholdCoin generates keys and secrets for a ThresholdCoin.
func generateThresholdCoin(nProcesses, threshold int) *globalThresholdCoin {
	var coeffs = make([]*big.Int, threshold)
	for i := 0; i < threshold; i++ {
		c, _ := rand.Int(rand.Reader, bn256.Order)
		coeffs[i] = c
	}
	secret := coeffs[threshold-1]

	globalVK := bn256.NewVerificationKey(secret)

	var wg sync.WaitGroup
	var sks = make([]*bn256.SecretKey, nProcesses)
	var vks = make([]*bn256.VerificationKey, nProcesses)

	for i := 0; i < nProcesses; i++ {
		wg.Add(1)
		go func(ind int) {
			defer wg.Done()
			secret := poly(coeffs, big.NewInt(int64(ind+1)))
			sks[ind] = bn256.NewSecretKey(secret)
			vks[ind] = bn256.NewVerificationKey(secret)
		}(i)
	}
	wg.Wait()

	return &globalThresholdCoin{
		threshold: threshold,
		globalVK:  globalVK,
		vks:       vks,
		sks:       sks,
	}
}

// CoinShare is a share of the coin owned by a process.
type CoinShare struct {
	pid int
	sgn *bn256.Signature
}

// Marshal returns byte representation of the given coin share in the following form
// (1) pid, 4 bytes as uint32
// (2) sgn
func (cs *CoinShare) Marshal() []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data[:4], uint32(cs.pid))
	data = append(data, cs.sgn.Marshal()...)
	return data
}

// Unmarshal reads a coin share from its byte representation.
func (cs *CoinShare) Unmarshal(data []byte) error {
	if len(data) < 4 {
		return errors.New("given data is too short")
	}
	pid := int(binary.LittleEndian.Uint32(data[0:4]))
	sgn := data[4:]
	cs.pid = pid
	decSgn, err := new(bn256.Signature).Unmarshal(sgn)
	if err != nil {
		return err
	}
	cs.sgn = decSgn
	return nil
}

// Coin is a result of merging CoinShares.
type Coin struct {
	sgn *bn256.Signature
}

// Unmarshal creates a coin from its byte representation.
func (c *Coin) Unmarshal(data []byte) error {
	if len(data) != bn256.SignatureLength {
		return errors.New("unmarshalling of coin failed. Wrong data length")
	}
	sgn := new(bn256.Signature)
	sgn, err := sgn.Unmarshal(data)
	if err != nil {
		return err
	}
	c.sgn = sgn
	return nil
}

// RandomBytes returns randomBytes from the coin.
func (c *Coin) RandomBytes() []byte {
	return c.sgn.Marshal()
}

// Toss returns a pseduorandom bit from a coin.
func (c *Coin) Toss() int {
	return int(c.sgn.Marshal()[0] & 1)
}

// CreateCoinShare creates a CoinShare for given process and nonce.
func (tc *ThresholdCoin) CreateCoinShare(nonce int) *CoinShare {
	return &CoinShare{
		pid: tc.pid,
		sgn: tc.sk.Sign(big.NewInt(int64(nonce)).Bytes()),
	}
}

// VerifyCoinShare verifies whether the given coin share is correct.
func (tc *ThresholdCoin) VerifyCoinShare(share *CoinShare, nonce int) bool {
	return tc.vks[share.pid].Verify(share.sgn, big.NewInt(int64(nonce)).Bytes())
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
// It returns
// the incorrect secret key when the pair of keys is invalid
// or
// nil when the keys are valid.
func (tc *ThresholdCoin) VerifySecretKey() *bn256.SecretKey {
	vk := tc.sk.VerificationKey()
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[tc.pid].Marshal()) != 1 {
		return tc.sk
	}
	return nil
}

// VerifyWrongSecretKeyProof verifies the proof given by a process that
// his secretKey is incorrect.
func (tc *ThresholdCoin) VerifyWrongSecretKeyProof(pid int, proof *bn256.SecretKey) bool {
	// Checking if the proof is equal to the stored secret key.
	// This is trival for now.
	// In the final version we will store encrypted secret keys.
	// Here we should encrypt proof and check
	// if the encrypted proof is equal to the stored encrypted secret key
	if subtle.ConstantTimeCompare(proof.Marshal(), tc.sks[pid].Marshal()) != 1 {
		return false
	}

	// Checking the proof
	vk := proof.VerificationKey()
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[pid].Marshal()) != 1 {
		return true
	}
	return false
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
		pid: cses[0].pid,
		sgn: sum,
	}
}

// CombineCoinShares combines the given shares into a Coin.
// It returns a Coin and a bool value indicating whether combining was successful or not.
func (tc *ThresholdCoin) CombineCoinShares(shares []*CoinShare) (*Coin, bool) {
	if len(shares) > tc.Threshold {
		shares = shares[:tc.Threshold]
	}
	if tc.Threshold != len(shares) {
		return nil, false
	}
	var points []int
	for _, sh := range shares {
		points = append(points, sh.pid)
	}

	var sum *bn256.Signature
	summands := make(chan *bn256.Signature)

	var wg sync.WaitGroup
	for _, sh := range shares {
		wg.Add(1)
		go func(ch *CoinShare) {
			defer wg.Done()
			summands <- bn256.MulSignature(ch.sgn, lagrange(points, ch.pid))
		}(sh)
	}
	go func() {
		wg.Wait()
		close(summands)
	}()

	for elem := range summands {
		sum = bn256.AddSignatures(sum, elem)
	}

	return &Coin{sgn: sum}, true
}

func lagrange(points []int, x int) *big.Int {
	num := big.NewInt(int64(1))
	den := big.NewInt(int64(1))
	for _, p := range points {
		if p == x {
			continue
		}
		num.Mul(num, big.NewInt(int64(0-p-1)))
		den.Mul(den, big.NewInt(int64(x-p)))
	}
	den.ModInverse(den, bn256.Order)
	num.Mul(num, den)
	num.Mod(num, bn256.Order)
	return num
}

func poly(coeffs []*big.Int, x *big.Int) *big.Int {
	ans := big.NewInt(int64(0))
	for _, c := range coeffs {
		ans.Mul(ans, x)
		ans.Add(ans, c)
		ans.Mod(ans, bn256.Order)
	}
	return ans
}
