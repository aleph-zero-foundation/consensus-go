package network_test

import (
	"bytes"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Multicast", func() {

	var (
		conns []Connection
		outs  []Connection
		mc    *Multicaster

		msg []byte
		N   int
	)

	Describe("using pipes", func() {
		Describe("multicasting", func() {
			BeforeEach(func() {
				N = 10
				conns = make([]Connection, N)
				outs = make([]Connection, N)
				for i := range conns {
					conns[i], outs[i] = tests.NewConnection()
				}
				mc = NewMulticaster(conns)
				msg = []byte("test")
			})
			It("Should deliver messages to all recipents", func() {
				var wg sync.WaitGroup
				for _, out := range outs {
					wg.Add(1)
					go func(o Connection) {
						defer wg.Done()
						b := make([]byte, len(msg))
						n, err := o.Read(b)
						Expect(n).To(Equal(len(msg)))
						Expect(err).NotTo(HaveOccurred())
						Expect(bytes.Equal(b, msg)).To(BeTrue())
					}(out)
				}
				n, err := mc.Write(msg)
				Expect(n).To(Equal(len(msg)))
				Expect(err).NotTo(HaveOccurred())
				wg.Wait()
			})
		})
	})
})
