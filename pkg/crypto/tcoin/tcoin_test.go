package tcoin_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcoin", func() {
	var n, t int
	var nonce int
	var gtc *GlobalThresholdCoin
	var tc []*ThresholdCoin
	var coinShares []*CoinShare
	Context("Between small number of processes", func() {
		BeforeEach(func() {
			n, t = 10, 3
			gtc = GenerateThresholdCoin(n, t)
			for i := 0; i < n; i++ {
				tc = append(tc, NewThresholdCoin(gtc, i))
			}
		})
		Describe("Coin shares", func() {
			BeforeEach(func() {
				nonce = 123
				coinShares = make([]*CoinShare, n)
				for i := 0; i < n; i++ {
					coinShares[i] = tc[i].CreateCoinShare(nonce)
				}
			})
			It("should be verified correctly", func() {
				Expect(tc[1].VerifyCoinShare(coinShares[2], nonce)).To(BeTrue())
				Expect(tc[1].VerifyCoinShare(coinShares[2], nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by t-parties", func() {
				c, ok := tc[0].CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				Expect(tc[0].VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tc[0].VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Shouldn't be correctly combined by t-1-parties", func() {
				_, ok := tc[0].CombineCoinShares(coinShares[:(t - 1)])
				Expect(ok).To(BeFalse())
			})
		})
	})
})
