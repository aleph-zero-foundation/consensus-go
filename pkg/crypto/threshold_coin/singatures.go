package signatures

import (
	"crypto/rand"
	"crypto/subtle"
	"golang.org/x/crypto/bn256"
	"math/big"
	"sync"
)

// VerificationKey is a vk
type VerificationKey struct {
	key *bn256.G2
}

// SecretKey is a sk
type SecretKey struct {
	key *big.Int
}

// Signature is a signature
type Signature []byte

var gen *bn256.G2 = new(bn256.G2).ScalarBaseMult(big.NewInt(int64(1)))

// Verify verifies the signature
func (vk *VerificationKey) Verify(s Signature, msg *big.Int) bool {
	sHash, ok := new(bn256.G1).Unmarshal(s)
	if !ok {
		return false
	}

	p1 := bn256.Pair(sHash, gen).Marshal()
	p2 := bn256.Pair(new(bn256.G1).ScalarBaseMult(msg), vk.key).Marshal()

	return subtle.ConstantTimeCompare(p1, p2) == 1
}

// Sign signs the msg
func (sk *SecretKey) Sign(msg *big.Int) Signature {
	msgHash := new(bn256.G1).ScalarBaseMult(msg)
	sgn := new(bn256.G1).ScalarMult(msgHash, sk.key)
	return sgn.Marshal()
}

func generateKeys(n, t int) ([]SecretKey, []VerificationKey, VerificationKey) {
	var coeffs = make([]*big.Int, t)
	for i := 0; i < t; i++ {
		c, _, _ := bn256.RandomG1(rand.Reader)
		coeffs[i] = c
	}
	secret := coeffs[t-1]

	vk := VerificationKey{
		key: new(bn256.G2).ScalarBaseMult(secret),
	}

	var wg sync.WaitGroup
	var sks = make([]SecretKey, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			sks[i] = SecretKey{
				key: poly(coeffs, big.NewInt(int64(i+1))),
			}
			wg.Done()
		}()
	}
	wg.Wait()

	var vks = make([]VerificationKey, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vks[i] = VerificationKey{
				key: new(bn256.G2).ScalarBaseMult(sks[i].key),
			}
		}()
	}
	return sks, vks, vk
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
	return num.Mul(num, den)
}

func combineShares(shares map[int]*big.Int) *big.Int {
	var points []int
	for i := range shares {
		points = append(points, i)
	}
	var summands = make([]*big.Int, len(points))

	var wg sync.WaitGroup
	for i, p := range points {
		wg.Add(1)
		go func() {
			defer wg.Done()
			summands[i] = big.NewInt(int64(0)).Mul(shares[p], lagrange(points, p))
		}()
	}
	wg.Wait()
	for i := 1; i < len(summands); i++ {
		summands[0].Add(summands[0], summands[i])
	}

	return summands[0].Mod(summands[0], bn256.Order)
}
