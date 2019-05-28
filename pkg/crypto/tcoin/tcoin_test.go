package tcoin_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcoin", func() {
	var n, t int
	var nonce int
	var tc *ThresholdCoin
	var coinShares []*CoinShare
	Context("Between small number of processes", func() {
		BeforeEach(func() {
			n, t = 10, 3
			tc = GenerateThresholdCoin(n, t)
		})
		Describe("Coin shares", func() {
			BeforeEach(func() {
				nonce = 123
				coinShares = make([]*CoinShare, n)
				for pid := 0; pid < n; pid++ {
					coinShares[pid] = tc.CreateCoinShare(pid, nonce)
				}
			})
			It("should be verified correctly", func() {
				Expect(tc.VerifyCoinShare(coinShares[1], nonce)).To(BeTrue())
				Expect(tc.VerifyCoinShare(coinShares[1], nonce+1)).To(BeFalse())
			})
			It("Should be correctly combined by t-parties", func() {
				c, ok := tc.CombineCoinShares(coinShares[:t])
				Expect(ok).To(BeTrue())
				Expect(tc.VerifyCoin(c, nonce)).To(BeTrue())
				Expect(tc.VerifyCoin(c, nonce+1)).To(BeFalse())
			})
			It("Shouldn't be correctly combined by t-1-parties", func() {
				_, ok := tc.CombineCoinShares(coinShares[:(t - 1)])
				Expect(ok).To(BeFalse())
			})
		})
	})
})
