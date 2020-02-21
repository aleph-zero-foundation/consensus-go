package beacon_test

import (
	"bytes"
	"encoding/binary"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/encrypt"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
	"gitlab.com/alephledger/core-go/pkg/crypto/tss"
)

var _ = Describe("Beacon", func() {
	var (
		n        uint16
		maxLevel int
		cnfs     []config.Config
		epoch    gomel.EpochID
		dags     []gomel.Dag
		rs       []gomel.RandomSource
		rsf      []gomel.RandomSourceFactory
		sks      []gomel.PrivateKey
		pks      []gomel.PublicKey
		sKeys    []*p2p.SecretKey
		pKeys    []*p2p.PublicKey
		p2pKeys  [][]encrypt.SymmetricKey
		err      error
		u        gomel.Unit
		rsData   []byte
	)

	BeforeEach(func() {
		n = 4
		epoch = gomel.EpochID(0)
		maxLevel = 13
		cnfs = make([]config.Config, n)
		dags = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)
		rsf = make([]gomel.RandomSourceFactory, n)
		sks = make([]gomel.PrivateKey, n)
		pks = make([]gomel.PublicKey, n)
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
			pks[pid], sks[pid], err = signing.GenerateKeys()
			Expect(err).NotTo(HaveOccurred())
			cnfs[pid] = config.Empty()
			cnfs[pid].Pid = pid
			cnfs[pid].NProc = n
			cnfs[pid].CanSkipLevel = true
			cnfs[pid].OrderStartLevel = 0
			cnfs[pid].Checks = append(cnfs[pid].Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
			cnfs[pid].PrivateKey = sks[pid]
			cnfs[pid].P2PSecretKey = sKeys[pid]
		}
		for pid := uint16(0); pid < n; pid++ {
			cnfs[pid].PublicKeys = pks
			cnfs[pid].P2PPublicKeys = pKeys
			dags[pid] = dag.New(cnfs[pid], epoch)
			rsf[pid], err = New(cnfs[pid])
			Expect(err).NotTo(HaveOccurred())
			rs[pid] = rsf[pid].NewRandomSource(dags[pid])
		}
	})
	Describe("Adding units", func() {
		BeforeEach(func() {
			// Generating very regular dag
			for level := 0; level < maxLevel; level++ {
				parents := make([]gomel.Unit, n)
				for creator := uint16(0); creator < n; creator++ {
					// create a unit
					if level == 0 {
						rsData, err = rsf[creator].DealingData(epoch)
						Expect(err).ToNot(HaveOccurred())
					} else {
						for pid := uint16(0); pid < n; pid++ {
							parents[pid] = dags[creator].UnitsOnLevel(level - 1).Get(pid)[0]
						}
						rsData, err = rs[creator].DataToInclude(parents, level)
						Expect(err).ToNot(HaveOccurred())
					}
					u = unit.New(creator, epoch, parents, level, core.Data{}, rsData, sks[creator])
					// add the unit to dags
					for pid := uint16(0); pid < n; pid++ {
						if level == 6 {
						}
						dags[pid].Insert(u)
					}
				}
			}
		})
		Context("that are dealing, but without a key included", func() {
			It("Should return an error", func() {
				u := dags[0].UnitsOnLevel(0).Get(0)[0]
				um := newUnitMock(u, []byte{})
				err := dags[0].Check(um)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(HavePrefix("Decoding key failed")))
			})
		})
		Context("that are voting", func() {
			Context("but have no votes", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(3).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(HavePrefix("votes wrongly encoded")))
				})
			})
			Context("but have one vote missing", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[0] = 0
					um := newUnitMock(u, votes)
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing vote"))
				})
			})
			Context("but have an incorrect vote", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(3).Get(0)[0]
					votes := u.RandomSourceData()
					votes[dags[0].NProc()-1] = 2
					// preparing fake proof
					pkFake, skFake, _ := p2p.GenerateKeys()
					proof := p2p.NewSharedSecret(skFake, pkFake)
					proofBytes := proof.Marshal()
					var buf bytes.Buffer
					binary.Write(&buf, binary.LittleEndian, uint16(len(proofBytes)))
					buf.Write(proofBytes)
					votes = append(votes, buf.Bytes()...)

					um := newUnitMock(u, votes)
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("the provided proof is incorrect"))
				})
			})
		})
		Context("that should contain shares", func() {
			Context("Without random source data", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(8).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("shares wrongly encoded"))
				})
			})
			Context("With missing shares", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(8).Get(0)[0]
					shares := make([]byte, dags[0].NProc())
					um := newUnitMock(u, shares)
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("missing share"))
				})
			})
			Context("With incorrect shares", func() {
				It("Should return an error", func() {
					u := dags[0].UnitsOnLevel(8).Get(0)[0]
					// taking shares of a unit of different level
					v := dags[0].UnitsOnLevel(9).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("invalid share"))
				})
			})
		})
	})
	Context("When a malicious process sends wrong key to one of the processes", func() {
		var maliciousNode uint16
		BeforeEach(func() {
			maliciousNode = uint16(2)
			dags[maliciousNode] = dag.New(cnfs[maliciousNode], epoch)
			rsf[maliciousNode] = &maliciousRandomSourceFactory{p2pKeys[maliciousNode]}
			rs[maliciousNode] = rsf[maliciousNode].NewRandomSource(dags[maliciousNode])
			rsData, err := rsf[maliciousNode].DealingData(epoch)
			Expect(err).ToNot(HaveOccurred())
			u = unit.New(maliciousNode, epoch, make([]gomel.Unit, n), 0, core.Data{}, rsData, sks[maliciousNode])
			// add the unit to dags
			for pid := uint16(0); pid < n; pid++ {
				if pid == maliciousNode {
					continue
				}
				dags[pid].Insert(u)
			}
			for level := 0; level < maxLevel; level++ {
				for creator := uint16(0); creator < n; creator++ {
					if creator == maliciousNode {
						continue
					}
					// create a unit
					if level == 0 {
					} else {
						parents := make([]gomel.Unit, n)
						for pid := uint16(0); pid < n; pid++ {
							parents[pid] = dags[creator].UnitsOnLevel(level - 1).Get(pid)[0]
						}
						rsData, err := rs[creator].DataToInclude(parents, level)
						Expect(err).ToNot(HaveOccurred())
						u = unit.New(creator, epoch, parents, level, core.Data{}, rsData, sks[creator])
					}
					// add the unit to dags
					for pid := uint16(0); pid < n; pid++ {
						if pid == maliciousNode {
							continue
						}
						dags[pid].Insert(u)
					}
				}
			}
		})
		It("Should produce a multikey which is the sum of keys of honest nodes", func() {
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
				subkeys := []*tss.ThresholdKey{}
				for i := uint16(0); i < n; i++ {
					if i == maliciousNode {
						continue
					}
					tk, _, _ := tss.Decode(dags[pid].UnitsOnLevel(0).Get(i)[0].RandomSourceData(), i, pid, p2pKeys[pid][i])
					subkeys = append(subkeys, tk)
				}
				multikey := tss.CreateWTK(subkeys, expectedShareProviders)

				expectedCoin := coin.NewFactory(pid, multikey)
				Expect(expectedCoin).To(Equal(obtainedCoin))
			}
		})
	})
})

type maliciousRandomSourceFactory struct {
	keys []encrypt.SymmetricKey
}

func (mrsf *maliciousRandomSourceFactory) NewRandomSource(_ gomel.Dag) gomel.RandomSource {
	return &maliciousRandomSource{}
}

func (mrsf *maliciousRandomSourceFactory) DealingData(_ gomel.EpochID) ([]byte, error) {
	nProc := uint16(len(mrsf.keys))
	gtk := tss.NewRandom(nProc, gomel.MinimalTrusted(nProc))
	tk, _ := gtk.Encrypt(mrsf.keys)
	encoded := tk.Encode()
	// forging last byte
	encoded[len(encoded)-1]++
	return encoded, nil
}

type maliciousRandomSource struct{}

func (mrs *maliciousRandomSource) RandomBytes(_ uint16, _ int) []byte { return nil }

func (mrs *maliciousRandomSource) DataToInclude(_ []gomel.Unit, _ int) ([]byte, error) {
	return nil, nil
}

type unitMock struct {
	gomel.Unit
	rsData []byte
}

func newUnitMock(u gomel.Unit, rsData []byte) *unitMock {
	return &unitMock{u, rsData}
}

func (um *unitMock) RandomSourceData() []byte {
	return um.rsData
}
