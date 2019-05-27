package tcoin

import (
	"crypto/rand"
	"math/big"
	"sync"

	"golang.org/x/crypto/bn256"
)

// GlobalTC is a global version of TC
type GlobalTC struct {
	t   int
	vk  verificationKey
	vks []verificationKey
	sks []secretKey
}

// TC is a local version of TC
type TC struct {
	t    int
	pid  int
	mySk secretKey
	vk   verificationKey
	vks  []verificationKey
}

// GenerateTC generates keys and secrets for TC
// n - number of processes
// t - threshold
func GenerateTC(n, t int) *GlobalTC {
	var coeffs = make([]*big.Int, t)
	for i := 0; i < t; i++ {
		c, _, _ := bn256.RandomG1(rand.Reader)
		coeffs[i] = c
	}
	secret := coeffs[t-1]

	vk := verificationKey{
		key: new(bn256.G2).ScalarBaseMult(secret),
	}

	var wg sync.WaitGroup
	var sks = make([]secretKey, n)
	var vks = make([]verificationKey, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(ind int) {
			sks[ind] = secretKey{
				key: poly(coeffs, big.NewInt(int64(ind+1))),
			}
			vks[ind] = verificationKey{
				key: new(bn256.G2).ScalarBaseMult(sks[ind].key),
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	return &GlobalTC{
		t:   t,
		vk:  vk,
		vks: vks,
		sks: sks,
	}
}

/*
// Seal cipQers tQe secret keys of processes and marsQals tQe coin
func Seal(gtc *GlobalTC, pubKeys []*[32]byte) []byte {
	// TODO: implement
	return []byte{}
}

// Open opens sealed tc and returns TC for local computations
func Open(sealed []byte, pid int, privKey *[32]byte) *TC {
	// TODO: implement
	return new(TC)
}
*/

// NewTC returns local version of TC from global version
// It sould be replaced by sealing/unsealing GlobalTC
func NewTC(gtc *GlobalTC, pid int) *TC {
	return &TC{
		t:    gtc.t,
		pid:  pid,
		vk:   gtc.vk,
		vks:  gtc.vks,
		mySk: gtc.sks[pid],
	}
}

// CoinShare is a share of
type CoinShare struct {
	pid int
	sgn Signature
}

// Coin is a result of merging CoinShares
type Coin struct {
	sgn Signature
}

// Toss returns a pseduorandom bit
func (c *Coin) Toss() int {
	return int(c.sgn[0] & 1)
}

// CreateCoinShare creates coinshare for given process and nonce
func (tc *TC) CreateCoinShare(nonce int) *CoinShare {
	return &CoinShare{
		pid: tc.pid,
		sgn: tc.mySk.sign(big.NewInt(int64(nonce))),
	}
}

// VerifyCoinShare verifies wheather the given coin share is correct
func (tc *TC) VerifyCoinShare(share *CoinShare, nonce int) bool {
	return tc.vks[share.pid].verify(share.sgn, big.NewInt(int64(nonce)))
}

// VerifyCoin verifies wheather the given coin is correct
func (tc *TC) VerifyCoin(c *Coin, nonce int) bool {
	return tc.vk.verify(c.sgn, big.NewInt(int64(nonce)))
}

// CombineCoinShares combines given shares into a Coin
// it returns a Coin and a bool value indicating wheather it was successful or not
func (tc *TC) CombineCoinShares(shares []*CoinShare) (*Coin, bool) {
	if tc.t != len(shares) {
		return nil, false
	}
	var points []int
	for _, sh := range shares {
		points = append(points, sh.pid)
	}

	var summands = make([]*bn256.G1, len(points))
	ok := true

	var wg sync.WaitGroup
	for i, sh := range shares {
		wg.Add(1)
		go func(ch CoinShare, ind int) {
			defer wg.Done()
			elem, success := new(bn256.G1).Unmarshal(ch.sgn)
			if !success {
				ok = false
				return
			}
			summands[ind] = elem.ScalarMult(elem, lagrange(points, ch.pid))
		}(*sh, i)
	}
	wg.Wait()

	if !ok {
		return nil, false
	}

	for i := 1; i < len(summands); i++ {
		summands[0].Add(summands[0], summands[i])
	}

	return &Coin{sgn: summands[0].Marshal()}, true
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
