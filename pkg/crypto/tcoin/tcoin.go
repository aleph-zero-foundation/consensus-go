package tcoin

import (
	"crypto/rand"
	"math/big"
	"sync"

	"golang.org/x/crypto/bn256"
)

// ThresholdCoin is a threshold coin as described in the whitepaper
// in the full version of the protocol localSKs should be sealed
type ThresholdCoin struct {
	Threshold int
	globalVK  verificationKey
	localVKs  []verificationKey
	localSKs  []secretKey
}

// GenerateThresholdCoin generates keys and secrets for ThresholdCoin
func GenerateThresholdCoin(nProcesses, threshold int) *ThresholdCoin {
	var coeffs = make([]*big.Int, threshold)
	for i := 0; i < threshold; i++ {
		c, _, _ := bn256.RandomG1(rand.Reader)
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

	return &ThresholdCoin{
		Threshold: threshold,
		globalVK:  globalVK,
		localVKs:  vks,
		localSKs:  sks,
	}
}

// CoinShare is a share of the coin owned by a process.
type CoinShare struct {
	pid int
	sgn signature
}

// Coin is a result of merging CoinShares
type Coin struct {
	sgn signature
}

// Toss returns a pseduorandom bit from a coin
func (c *Coin) Toss() int {
	return int(c.sgn[0] & 1)
}

// CreateCoinShare creates a CoinShare for given process and nonce
func (tc *ThresholdCoin) CreateCoinShare(pid, nonce int) *CoinShare {
	return &CoinShare{
		pid: pid,
		sgn: tc.localSKs[pid].sign(big.NewInt(int64(nonce))),
	}
}

// VerifyCoinShare verifies wheather the given coin share is correct
func (tc *ThresholdCoin) VerifyCoinShare(share *CoinShare, nonce int) bool {
	return tc.localVKs[share.pid].verify(share.sgn, big.NewInt(int64(nonce)))
}

// VerifyCoin verifies wheather the given coin is correct
func (tc *ThresholdCoin) VerifyCoin(c *Coin, nonce int) bool {
	return tc.globalVK.verify(c.sgn, big.NewInt(int64(nonce)))
}

// CombineCoinShares combines given shares into a Coin
// it returns a Coin and a bool value indicating wheather combining was successful or not
func (tc *ThresholdCoin) CombineCoinShares(shares []*CoinShare) (*Coin, bool) {
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
	for i, sh := range shares {
		wg.Add(1)
		go func(ch *CoinShare, ind int) {
			defer wg.Done()
			elem, success := new(bn256.G1).Unmarshal(ch.sgn)
			if !success {
				ok = false
				return
			}
			summands <- elem.ScalarMult(elem, lagrange(points, ch.pid))
		}(sh, i)
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
