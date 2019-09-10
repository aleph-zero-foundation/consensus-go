package gossip_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
)

var _ = Describe("Peer Source", func() {
	var (
		ps  PeerSource
		n   uint16
		pid uint16
		ch  chan uint16
	)

	Describe("Default", func() {
		BeforeEach(func() {
			n, pid = 10, 7
			ps = NewDefaultPeerSource(n, pid)
		})
		Context("NextPeer", func() {
			It("Should return value less than n, different than pid", func() {
				next := ps.NextPeer()
				Expect(next).To(BeNumerically("<", n))
				Expect(next).NotTo(BeNumerically("==", pid))
			})
			It("Should return all values different than pid, after finite number of calls", func() {
				values := make(map[uint16]bool)
				for len(values) != int(n-1) {
					values[ps.NextPeer()] = true
				}
				_, ok := values[pid]
				Expect(ok).To(BeFalse())
			})
		})
	})
	Describe("Channel", func() {
		BeforeEach(func() {
			ch = make(chan uint16)
			ps = NewChanPeerSource(ch)
		})
		Context("NextPeer", func() {
			It("Should read the value from the channel", func() {
				go func() {
					ch <- 123
				}()
				next := ps.NextPeer()
				Expect(next).To(Equal(uint16(123)))
			})
		})
	})
	Describe("Mixed", func() {
		BeforeEach(func() {
			ch = make(chan uint16, 1)
			n, pid = 10, 7
			ps = NewMixedPeerSource(n, pid, ch)
		})
		Context("NextPeer", func() {
			Context("When the channel is empty", func() {
				It("Should return value less than n, different than pid", func() {
					next := ps.NextPeer()
					Expect(next).To(BeNumerically("<", n))
					Expect(next).NotTo(BeNumerically("==", pid))
				})
				It("Should return all values different than pid, after finite number of calls", func() {
					values := make(map[uint16]bool)
					for len(values) != int(n-1) {
						values[ps.NextPeer()] = true
					}
					_, ok := values[pid]
					Expect(ok).To(BeFalse())
				})
			})
			Context("When the channel is non-empty", func() {
				It("Should read from the value from the channel", func() {
					ch <- 123
					Expect(ps.NextPeer()).To(Equal(uint16(123)))
				})
			})
		})
	})
})
