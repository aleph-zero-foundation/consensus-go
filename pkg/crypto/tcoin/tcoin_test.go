package tcoin_test

import (
	"math/big"
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcoin", func() {
	var n, t uint16
	var nonce int
	var tcs []*ThresholdCoin
	var coinShares []*CoinShare
	var coeffs []*big.Int
	var tcs1, tcs2 []*ThresholdCoin

	Context("Based on random polynomial", func() {
		BeforeEach(func() {
			n, t = 10, 4
			coeffs = make([]*big.Int, t)
			tcs = make([]*ThresholdCoin, n)
			rnd := rand.New(rand.NewSource(123))
			for i := 0; i < t; i++ {
				coeffs[i] = big.NewInt(0).Rand(rnd, bn256.Order)
			}
			for i := 0; i < n; i++ {
				tcs[i] = New(n, i, coeffs)
			}
			nonce = 123
			coinShares = make([]*CoinShare, n)
			for i := 0; i < n; i++ {
				coinShares[i] = tcs[i].CreateCoinShare(nonce)
			}
		})
		It("Should be verified by a poly verifier", func() {
			pv := bn256.NewPolyVerifier(n, t)
			for i := 0; i < n; i++ {
				Expect(tcs[i].PolyVerify(pv)).To(BeTrue())
			}
		})
		It("Should have correct secret key", func() {
			Expect(tcs[0].VerifySecretKey()).To(BeNil())
		})
	})
	Context("Between small number of processes", func() {
		Describe("Coin shares", func() {
			BeforeEach(func() {
				n, t = 10, 4
				dealt := Deal(n, t)
				tcs = make([]*ThresholdCoin, n)
				for i := uint16(0); i < n; i++ {
					tc, err := Decode(dealt, i)
					Expect(err).NotTo(HaveOccurred())
					tcs[i] = tc
				}
				nonce = 123
				coinShares = make([]*CoinShare, n)
				for i := uint16(0); i < n; i++ {
					coinShares[i] = tcs[i].CreateCoinShare(nonce)
				}
			})
			It("should be verified correctly", func() {
				Expect(tcs[2].VerifyCoinShare(coinShares[1], nonce)).To(BeTrue())
				Expect(tcs[2].VerifyCoinShare(coinShares[1], nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by t-parties", func() {
				c, ok := tcs[0].CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by more than t-parties", func() {
				c, ok := tcs[0].CombineCoinShares(coinShares)
				Expect(ok).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tcs[0].VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Should be combined to the same coin by two different sets of t-parties", func() {
				c1, ok := tcs[0].CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				c2, ok := tcs[0].CombineCoinShares(coinShares[(n - t):])
				Expect(ok).To(BeTrue())
				Expect(c1.RandomBytes()).To(Equal(c2.RandomBytes()))
				Expect(c1.Toss()).To(Equal(c2.Toss()))
				Expect(c1.Toss()).To(SatisfyAny(Equal(0), Equal(1)))
			})
			It("Shouldn't be correctly combined by t-1-parties", func() {
				_, ok := tcs[0].CombineCoinShares(coinShares[:(t - 1)])
				Expect(ok).To(BeFalse())
			})
			It("Should be marshalled and unmarshalled correctly", func() {
				for i := uint16(0); i < n; i++ {
					csMarshalled := coinShares[i].Marshal()
					var cs = new(CoinShare)
					err := cs.Unmarshal(csMarshalled)
					Expect(err).NotTo(HaveOccurred())
					Expect(tcs[0].VerifyCoinShare(cs, nonce)).To(BeTrue())
				}
			})
		})
	})

	Context("Multicoin", func() {
		BeforeEach(func() {
			n, t = 10, 4
			dealt1 := Deal(n, t)
			dealt2 := Deal(n, t)
			tcs = make([]*ThresholdCoin, n)
			tcs1 = make([]*ThresholdCoin, n)
			tcs2 = make([]*ThresholdCoin, n)

			for i := 0; i < n; i++ {
				tc, err := Decode(dealt1, i)
				Expect(err).NotTo(HaveOccurred())
				tcs1[i] = tc
				tc, err = Decode(dealt2, i)
				Expect(err).NotTo(HaveOccurred())
				tcs2[i] = tc
				tcs[i] = CreateMulticoin([]*ThresholdCoin{tcs1[i], tcs2[i]})
			}
			nonce = 123
			coinShares = make([]*CoinShare, n)
			for i := 0; i < n; i++ {
				coinShares[i] = tcs[i].CreateCoinShare(nonce)
			}
		})
		Describe("coin shares", func() {
			It("should be the sum of coin shares among single coins", func() {
				cs := SumShares([]*CoinShare{tcs1[0].CreateCoinShare(nonce), tcs2[0].CreateCoinShare(nonce)})
				Expect(cs.Marshal()).To(Equal(coinShares[0].Marshal()))
			})
		})
	})
})
