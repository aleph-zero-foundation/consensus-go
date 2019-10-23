package beacon_test

import (
	"bytes"
	"encoding/binary"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Beacon", func() {
	var (
		n        uint16
		maxLevel int
		dag      []gomel.Dag
		rs       []gomel.RandomSource
		sKeys    []*p2p.SecretKey
		pKeys    []*p2p.PublicKey
		p2pKeys  [][]encrypt.SymmetricKey
		err      error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 13
		dag = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)
		sKeys = make([]*p2p.SecretKey, n)
		pKeys = make([]*p2p.PublicKey, n)
		p2pKeys = make([][]encrypt.SymmetricKey, n)
		for i := uint16(0); i < n; i++ {
			pKeys[i], sKeys[i], _ = p2p.GenerateKeys()
		}
		for i := uint16(0); i < n; i++ {
			p2pKeys[i], _ = p2p.Keys(sKeys[i], pKeys, i)
		}
		for pid := uint16(0); pid < n; pid++ {
			dag[pid], err = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs[pid], err = New(pid, pKeys, sKeys[pid])
			Expect(err).NotTo(HaveOccurred())
			dag[pid] = rs[pid].Bind(dag[pid])
		}
	})

	Describe("Adding units", func() {
		BeforeEach(func() {
			// Generating very regular dag
			for level := 0; level < maxLevel; level++ {
				for creator := uint16(0); creator < n; creator++ {
					pu, _, err := creating.NewUnit(dag[creator], creator, []byte{}, rs[creator], false)
					Expect(err).NotTo(HaveOccurred())
					for pid := uint16(0); pid < n; pid++ {
						_, err = gomel.AddUnit(dag[pid], pu)
						Expect(err).NotTo(HaveOccurred())
					}
				}
			}
		})
		Context("that are dealing, but without a tcoin included", func() {
			It("Should return an error", func() {
				u := dag[0].PrimeUnits(0).Get(0)[0]
				um := newUnitMock(u, []byte{})
				err := dag[0].Check(um)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(HavePrefix("Decoding tcoin failed")))
			})
		})
		Context("that are voting", func() {
			Context("but have no votes", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(HavePrefix("votes wrongly encoded")))
				})
			})
			Context("but have one vote missing", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[0] = 0
					um := newUnitMock(u, votes)
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing vote"))
				})
			})
			Context("but have an incorrect vote", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[dag[0].NProc()-1] = 2
					// preparing fake proof
					pkFake, skFake, _ := p2p.GenerateKeys()
					proof := p2p.NewSharedSecret(skFake, pkFake)
					proofBytes := proof.Marshal()
					var buf bytes.Buffer
					binary.Write(&buf, binary.LittleEndian, uint16(len(proofBytes)))
					buf.Write(proofBytes)
					votes = append(votes, buf.Bytes()...)

					um := newUnitMock(u, votes)
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("the provided proof is incorrect"))
				})
			})
		})
		Context("that should contain shares", func() {
			Context("Without random source data", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("cses wrongly encoded"))
				})
			})
			Context("With missing shares", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					shares := make([]byte, dag[0].NProc())
					um := newUnitMock(u, shares)
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing share"))
				})
			})
			Context("With incorrect shares", func() {
				It("Should return an error", func() {
					u := dag[0].PrimeUnits(8).Get(0)[0]
					// taking shares of a unit of different level
					v := dag[0].PrimeUnits(9).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("invalid share"))
				})
			})
		})
	})

	Context("When a malicious process sends wrong tcoin to one of the processes", func() {
		var maliciousNode uint16
		BeforeEach(func() {
			maliciousNode = uint16(2)
			dag[maliciousNode], err = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs[maliciousNode] = &maliciousDealerSource{p2pKeys[maliciousNode]}
			dag[maliciousNode] = rs[maliciousNode].Bind(dag[maliciousNode])
			pu, _, err := creating.NewUnit(dag[maliciousNode], maliciousNode, []byte{}, rs[maliciousNode], false)

			for pid := uint16(0); pid < n; pid++ {
				if pid == maliciousNode {
					continue
				}
				_, err = gomel.AddUnit(dag[pid], pu)
				Expect(err).NotTo(HaveOccurred())
			}
			for level := 0; level < maxLevel; level++ {
				for creator := uint16(0); creator < n; creator++ {
					if creator == maliciousNode {
						continue
					}
					pu, _, err := creating.NewUnit(dag[creator], creator, []byte{}, rs[creator], false)
					Expect(err).NotTo(HaveOccurred())
					Expect(err).NotTo(HaveOccurred())
					for pid := uint16(0); pid < n; pid++ {
						if pid == maliciousNode {
							continue
						}
						_, err = gomel.AddUnit(dag[pid], pu)
						Expect(err).NotTo(HaveOccurred())
					}
				}
			}
		})
		It("Should produce a multicoin which is the sum of coins of honest nodes", func() {
			head := uint16(1)
			expectedShareProviders := map[uint16]bool{}
			for pid := uint16(0); pid < n; pid++ {
				if pid == maliciousNode {
					continue
				}
				expectedShareProviders[pid] = true
			}

			for pid := uint16(0); pid < n; pid++ {
				if pid == maliciousNode {
					continue
				}
				obtainedCoin := rs[pid].(*Beacon).GetCoin(head)
				subcoins := []*tcoin.ThresholdCoin{}
				for i := uint16(0); i < n; i++ {
					if i == maliciousNode {
						continue
					}
					tc, _, _ := tcoin.Decode(dag[pid].PrimeUnits(0).Get(i)[0].RandomSourceData(), i, pid, p2pKeys[pid][i])
					subcoins = append(subcoins, tc)
				}
				multicoin := tcoin.CreateMulticoin(subcoins)

				expectedCoin := coin.New(n, pid, multicoin, expectedShareProviders)
				Expect(expectedCoin).To(Equal(obtainedCoin))
			}
		})
	})
})

type maliciousDealerSource struct {
	keys []encrypt.SymmetricKey
}

func (ms *maliciousDealerSource) Bind(dag gomel.Dag) gomel.Dag {
	return dag
}

func (ms *maliciousDealerSource) RandomBytes(_ uint16, _ int) []byte {
	return []byte{}
}

func (ms *maliciousDealerSource) DataToInclude(creator uint16, parents []gomel.Unit, level int) ([]byte, error) {
	if level == 0 {
		nProc := uint16(len(ms.keys))
		gtc := tcoin.NewRandomGlobal(nProc, gomel.MinimalTrusted(nProc))
		tc, _ := gtc.Encrypt(ms.keys)
		encoded := tc.Encode()
		// forging last byte
		encoded[len(encoded)-1]++
		return encoded, nil
	}
	return nil, nil
}

type unitMock struct {
	u      gomel.Unit
	rsData []byte
}

func newUnitMock(u gomel.Unit, rsData []byte) *unitMock {
	return &unitMock{
		u:      u,
		rsData: rsData,
	}
}

func (um *unitMock) Creator() uint16 {
	return um.u.Creator()
}

func (um *unitMock) Signature() gomel.Signature {
	return um.u.Signature()
}

func (um *unitMock) Hash() *gomel.Hash {
	return um.u.Hash()
}

func (um *unitMock) View() *gomel.Crown {
	return um.u.View()
}

func (um *unitMock) Data() gomel.Data {
	return um.u.Data()
}

func (um *unitMock) RandomSourceData() []byte {
	return um.rsData
}

func (um *unitMock) Height() int {
	return um.u.Height()
}

func (um *unitMock) Parents() []gomel.Unit {
	return um.u.Parents()
}

func (um *unitMock) Level() int {
	return um.u.Level()
}

func (um *unitMock) Above(v gomel.Unit) bool {
	return um.u.Above(v)
}

func (um *unitMock) Floor() [][]gomel.Unit {
	return um.u.Floor()
}
