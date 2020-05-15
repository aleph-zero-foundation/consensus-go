package handshake_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

var _ = Describe("Greeting", func() {

	var (
		servs []network.Server
	)

	BeforeEach(func() {
		servs = ctests.NewNetwork(2, time.Second)
	})

	Context("correctly", func() {

		It("should send the information", func() {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				conn, err := servs[1].Dial(0)
				Expect(err).NotTo(HaveOccurred())
				Expect(Greet(conn, 1, 2)).To(Succeed())
				wg.Done()
			}()
			go func() {
				conn, err := servs[0].Listen()
				Expect(err).NotTo(HaveOccurred())
				pid, sid, err := AcceptGreeting(conn)
				Expect(err).NotTo(HaveOccurred())
				Expect(pid).To(BeNumerically("==", 1))
				Expect(sid).To(BeNumerically("==", 2))
				wg.Done()
			}()
			wg.Wait()
		})

	})

})
