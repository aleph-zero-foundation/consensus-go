package tcoin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"github.com/cloudflare/bn256"
)

// globalThresholdCoin is threshold coin generated by a dealer
// It contains secret keys of all processes
// In the full version of the protocol secretKeys should be sealed
type globalThresholdCoin struct {
	threshold int
	globalVK  verificationKey
	vks       []verificationKey
	sks       []secretKey
}

// ThresholdCoin is a threshold coin owned by one of the parties
// It contains pid of the owner
// sk is a secretKey of the owner
type ThresholdCoin struct {
	Threshold int
	pid       int
	globalVK  verificationKey
	vks       []verificationKey
	sk        secretKey
	// TODO: sks should be encrypted
	sks []secretKey
}

// Deal returns byte representation of a threshold coin
// with given threshold and number of processes
func Deal(nProcesses, threshold int) []byte {
	return generateThresholdCoin(nProcesses, threshold).encode()
}

// encode returns byte representation of the given gtc in the following form
// (1) threshold, 1 byte as uint32
// (2) length of marshalled globalVK, 1 byte as uint32
// (3) marshalled globalVK
// (4) len(vks), 1 byte as uint32
// (5) Marshalled vks in the form
//     a) length of marshalled vk
//     b) marshalled vk
// (6) Marshalled sks in the form
//     a) length of marshalled sk
//     b) marshalled sk
func (gtc *globalThresholdCoin) encode() []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[:4], uint32(gtc.threshold))

	globalVKMarshalled := gtc.globalVK.key.Marshal()
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(globalVKMarshalled)))
	data = append(data, globalVKMarshalled...)

	dataLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataLen[:], uint32(len(gtc.vks)))
	data = append(data, dataLen...)

	for _, vk := range gtc.vks {
		vkMarshalled := vk.key.Marshal()
		binary.LittleEndian.PutUint32(dataLen, uint32(len(vkMarshalled)))
		data = append(data, dataLen...)
		data = append(data, vkMarshalled...)
	}

	for _, sk := range gtc.sks {
		skMarshalled := sk.key.Bytes()
		binary.LittleEndian.PutUint32(dataLen, uint32(len(skMarshalled)))
		data = append(data, dataLen...)
		data = append(data, skMarshalled...)
	}
	return data
}

// Decode creates tc for given pid from byte representation of gtc.
func Decode(data []byte, pid int) (*ThresholdCoin, error) {
	ind := 0
	threshold := int(binary.LittleEndian.Uint32(data[:(ind + 4)]))
	ind += 4

	gvkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	key := new(bn256.G2)
	_, err := key.Unmarshal(data[ind:(ind + gvkLen)])
	if err != nil {
		return nil, errors.New("unmarshal of globalVK failed")
	}
	ind += gvkLen
	globalVK := verificationKey{
		key: key,
	}

	nProcesses := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
	ind += 4
	vks := make([]verificationKey, nProcesses)
	for i := range vks {
		vkLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		key := new(bn256.G2)
		_, err := key.Unmarshal(data[ind:(ind + vkLen)])
		if err != nil {
			return nil, errors.New("unmarshal of vk failed")
		}
		ind += vkLen
		vks[i] = verificationKey{
			key: key,
		}
	}
	sks := make([]secretKey, nProcesses)
	for i := range sks {
		skLen := int(binary.LittleEndian.Uint32(data[ind:(ind + 4)]))
		ind += 4
		key := big.NewInt(int64(0)).SetBytes(data[ind:(ind + skLen)])
		ind += skLen
		sks[i] = secretKey{
			key: key,
		}
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

// CreateMulticoin generates a multiCoin for given ThresholdCoins
// i.e. a ThresholdCoin which corresponds to the sum of polynomials
// which are defining given ThresholdCoins
// It assignes the multicoin to a given pid
// We assume that:
// (0) tcs is a non-empty slice
// (1) the threshold is the same for all given thresholdCoins
// (2) the thresholdCoins were created by different processes
func CreateMulticoin(tcs []*ThresholdCoin, pid int) *ThresholdCoin {
	n := len(tcs[0].vks)
	var result = ThresholdCoin{
		Threshold: tcs[0].Threshold,
		pid:       pid,
		sk:        secretKey{key: big.NewInt(0)},
		globalVK:  verificationKey{key: new(bn256.G2)},
	}
	vks := make([]verificationKey, n)
	for i := range vks {
		vks[i] = verificationKey{key: new(bn256.G2)}
	}
	result.vks = vks
	// TODO: we can use concurrency as we have multiple independent additions
	for _, tc := range tcs {
		result.sk.key.Add(result.sk.key, tc.sk.key)
		result.globalVK.key.Add(result.globalVK.key, tc.globalVK.key)
		for i, vk := range tc.vks {
			result.vks[i].key.Add(result.vks[i].key, vk.key)
		}
	}
	result.sk.key.Mod(result.sk.key, bn256.Order)
	return &result
}

// generateThresholdCoin generates keys and secrets for ThresholdCoin
func generateThresholdCoin(nProcesses, threshold int) *globalThresholdCoin {
	var coeffs = make([]*big.Int, threshold)
	for i := 0; i < threshold; i++ {
		c, _ := rand.Int(rand.Reader, bn256.Order)
		coeffs[i] = c
	}
	secret := coeffs[threshold-1]

	globalVK := verificationKey{
		key: new(bn256.G2).ScalarBaseMult(secret),
	}

	var wg sync.WaitGroup
	var sks = make([]secretKey, nProcesses)
	var vks = make([]verificationKey, nProcesses)

	for i := 0; i < nProcesses; i++ {
		wg.Add(1)
		go func(ind int) {
			defer wg.Done()
			sks[ind] = secretKey{
				key: poly(coeffs, big.NewInt(int64(ind+1))),
			}
			vks[ind] = verificationKey{
				key: new(bn256.G2).ScalarBaseMult(sks[ind].key),
			}
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
	sgn signature
}

// Marshal returns byte representation of the given cs in the following form
// (1) pid, 1 byte as uint32
// (2) sgn
func (cs *CoinShare) Marshal() []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data[:4], uint32(cs.pid))
	data = append(data, cs.sgn...)
	return data
}

// Unmarshal reads cs from its byte representation
func (cs *CoinShare) Unmarshal(data []byte) error {
	if len(data) < 4 {
		return errors.New("given data is too short")
	}
	pid := int(binary.LittleEndian.Uint32(data[0:4]))
	sgn := data[4:]
	cs.pid = pid
	cs.sgn = sgn
	return nil
}

// Coin is a result of merging CoinShares
type Coin struct {
	sgn signature
}

// RandomBytes returns randomBytes from the coin
func (c *Coin) RandomBytes() []byte {
	return c.sgn
}

// Toss returns a pseduorandom bit from a coin
func (c *Coin) Toss() int {
	return int(c.sgn[0] & 1)
}

// CreateCoinShare creates a CoinShare for given process and nonce
func (tc *ThresholdCoin) CreateCoinShare(nonce int) *CoinShare {
	return &CoinShare{
		pid: tc.pid,
		sgn: tc.sk.sign(big.NewInt(int64(nonce))),
	}
}

// VerifyCoinShare verifies wheather the given coin share is correct
func (tc *ThresholdCoin) VerifyCoinShare(share *CoinShare, nonce int) bool {
	return tc.vks[share.pid].verify(share.sgn, big.NewInt(int64(nonce)))
}

// VerifyCoin verifies wheather the given coin is correct
func (tc *ThresholdCoin) VerifyCoin(c *Coin, nonce int) bool {
	return tc.globalVK.verify(c.sgn, big.NewInt(int64(nonce)))
}

// PolyVerify uses given polyVerifier to verify if the vks form
// a polynomial sequence
func (tc *ThresholdCoin) PolyVerify(pv PolyVerifier) bool {
	elems := make([]*bn256.G2, len(tc.vks))
	for i, vk := range tc.vks {
		elems[i] = vk.key
	}
	return pv.Verify(elems)
}

// VerifySecretKey checks if the verificationKey and secretKey form a valid pair
// it returns
// - the incorrect secret key when the pair of keys is invalid
// or
// - nil when the keys are valid
func (tc *ThresholdCoin) VerifySecretKey() *big.Int {
	vk := new(bn256.G2).ScalarBaseMult(tc.sk.key)
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[tc.pid].key.Marshal()) != 1 {
		return tc.sk.key
	}
	return nil
}

// VerifyWrongSecretKeyProof verifies proof given by a process that
// his secretKey is incorrect
func (tc *ThresholdCoin) VerifyWrongSecretKeyProof(pid int, proof *big.Int) bool {
	// Checking if the proof is equal to the stored secret key.
	// This is trival for now.
	// In the final version we will store encrypted secret keys.
	// Here we should encrypt proof and check
	// if the encrypted proof is equal to the stored encrypted secret key
	if proof.Cmp(tc.sks[pid].key) != 0 {
		return false
	}

	// Checking the proof
	vk := new(bn256.G2).ScalarBaseMult(proof)
	if subtle.ConstantTimeCompare(vk.Marshal(), tc.vks[pid].key.Marshal()) != 1 {
		return true
	}
	return false
}

// CombineCoinShares combines given shares into a Coin
// it returns a Coin and a bool value indicating wheather combining was successful or not
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

	sum := new(bn256.G1)
	summands := make(chan *bn256.G1)
	ok := true

	var wg sync.WaitGroup
	for _, sh := range shares {
		wg.Add(1)
		go func(ch *CoinShare) {
			defer wg.Done()
			elem := new(bn256.G1)
			_, err := elem.Unmarshal(ch.sgn)
			if err != nil {
				ok = false
				return
			}
			summands <- elem.ScalarMult(elem, lagrange(points, ch.pid))
		}(sh)
	}
	go func() {
		wg.Wait()
		close(summands)
	}()

	for elem := range summands {
		sum.Add(sum, elem)
	}

	if !ok {
		return nil, false
	}

	return &Coin{sgn: sum.Marshal()}, true
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
