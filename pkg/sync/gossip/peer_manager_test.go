package gossip

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Peer Source", func() {
	var (
		pm      *peerManager
		nProc   uint16
		pid     uint16
		toCheck uint16
		idle    int
	)

	Describe("Peer Manager", func() {
		BeforeEach(func() {
			nProc = 16
			pid = 7
			idle = 2
			pm = newPeerManager(nProc, pid, idle)
		})
		Context("NextPeer", func() {
			It("Should return value less than nProc, different than pid", func() {
				next, _ := pm.nextPeer()
				pm.done(next)
				Expect(next).To(BeNumerically("<", nProc))
				Expect(next).NotTo(BeNumerically("==", pid))
			})
			It("Should return all values different than pid, after finite number of calls", func() {
				values := make(map[uint16]bool)
				for len(values) != int(nProc-1) {
					next, _ := pm.nextPeer()
					pm.done(next)
					values[next] = true
				}
				_, ok := values[pid]
				Expect(ok).To(BeFalse())
			})
			It("Should prioritize requests over random peers", func() {
				var wg sync.WaitGroup
				toRequest := uint16(13)
				// first drain all the idle tokens
				for i := 0; i < idle; i++ {
					next, _ := pm.nextPeer()
					// make sure we did not block toRequest pid
					if next == toRequest {
						pm.done(next)
						i--
					}
				}
				// start the goroutine waiting for the next pid
				wg.Add(1)
				go func() {
					toCheck, _ = pm.nextPeer()
					wg.Done()
				}()
				// request the pid
				pm.request(toRequest)
				wg.Wait()
				Expect(toCheck).To(BeNumerically("==", toRequest))
			})

		})
	})
})
