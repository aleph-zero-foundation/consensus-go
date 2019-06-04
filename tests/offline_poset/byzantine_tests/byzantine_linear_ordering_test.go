package byzantine_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"

	"golang.org/x/crypto/sha3"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/tests/offline_poset"
)

const (
	forkingPrimes = int(1000)
	floodingLevel = 10
)

// Implementation of the Preunit interface that allows to create arbitrary forks without changing units content
type preunitWithNonce struct {
	gomel.Preunit
	nounce uint64
	hash   gomel.Hash
}

func toBytes(data interface{}) []byte {
	var newData bytes.Buffer
	binary.Write(&newData, binary.LittleEndian, data)
	return newData.Bytes()
}

func (pu *preunitWithNonce) computeHash() {
	hash := *pu.Preunit.Hash()

	var data bytes.Buffer
	data.Write(hash[:])
	data.Write(toBytes(pu.nounce))
	sha3.ShakeSum256(pu.hash[:len(pu.hash)], data.Bytes())
}

func NewPreunitWithNounce(preunit gomel.Preunit, nounce uint64) *preunitWithNonce {
	pu := creating.NewPreunit(preunit.Creator(), preunit.Parents(), nil)

	result := preunitWithNonce{Preunit: pu, nounce: nounce}
	result.computeHash()
	return &result
}

func (pu *preunitWithNonce) Hash() *gomel.Hash {
	return &pu.hash
}

func createForks(preunit gomel.Preunit, privKey gomel.PrivateKey, count int) []gomel.Preunit {
	result := make([]gomel.Preunit, 0, count)
	created := map[gomel.Hash]bool{}
	for nounce := uint64(0); len(result) < count; nounce++ {
		fork := NewPreunitWithNounce(preunit, nounce)
		if created[*fork.Hash()] {
			continue
		}
		fork.SetSignature(privKey.Sign(fork))
		result = append(result, fork)
		created[*fork.Hash()] = true
	}
	return result
}

func computeMaxPossibleNumberOfByzantineProcesses(nProc int) int {
	max := nProc / 3
	if 3*max < nProc {
		return max
	} else {
		return max - 1
	}
}

func getRandomListOfByzantinePosets(n int) []int {
	byzProcesses := computeMaxPossibleNumberOfByzantineProcesses(n)
	return rand.Perm(byzProcesses)[:byzProcesses]
}

func newTriggeredAdder(triggerCondition func(unit gomel.Unit) bool, wrappedHandler offline_poset.AddingHandler) offline_poset.AddingHandler {

	return func(posets []*growing.Poset, unit gomel.Preunit) error {
		newUnit, err := offline_poset.AddToPosets(unit, posets)
		if err != nil {
			return err
		}
		if triggerCondition(newUnit) {
			return wrappedHandler(posets, unit)
		}
		return nil

	}
}

func newSimpleForkingAdder(forkingLevel int, privKeys []gomel.PrivateKey, byzantinePosets []int) offline_poset.AddingHandler {
	alreadyForked := make(map[int]bool, len(byzantinePosets))
	for _, posetID := range byzantinePosets {
		alreadyForked[posetID] = false
	}

	return newTriggeredAdder(
		func(unit gomel.Unit) bool {
			val, ok := alreadyForked[unit.Creator()]
			if ok && !val && unit.Level() >= forkingLevel {
				return true
			}
			return false
		},

		func(posets []*growing.Poset, unit gomel.Preunit) error {
			fmt.Println("simple forking behavior triggered")
			units := createForks(unit, privKeys[unit.Creator()], 2)
			err := offline_poset.AddUnitsToPosetsInRandomOrder(units, posets)
			if err != nil {
				return err
			}
			alreadyForked[unit.Creator()] = true
			fmt.Println("simple fork created at level", forkingLevel)
			return nil
		},
	)
}

func newPrimeFloodAdder(floodingLevel int, numberOfPrimes int, privKeys []gomel.PrivateKey, byzantinePosets []int) offline_poset.AddingHandler {
	alreadyFlooded := make(map[int]bool, len(byzantinePosets))
	for _, posetID := range byzantinePosets {
		alreadyFlooded[posetID] = false
	}

	return newTriggeredAdder(
		func(unit gomel.Unit) bool {
			val, ok := alreadyFlooded[unit.Creator()]
			if ok && !val && unit.Level() >= floodingLevel && gomel.Prime(unit) {
				return true
			}
			return false
		},

		func(posets []*growing.Poset, unit gomel.Preunit) error {
			fmt.Println("Prime flooding started")
			for _, unit := range createForks(unit, privKeys[unit.Creator()], numberOfPrimes) {
				if _, err := offline_poset.AddToPosets(unit, posets); err != nil {
					return err
				}
			}
			alreadyFlooded[unit.Creator()] = true
			fmt.Println("Prime flooding finished")
			return nil
		},
	)
}

func newRandomForkingAdder(byzantinePosets []int, forkProbability int, privKeys []gomel.PrivateKey) offline_poset.AddingHandler {
	forkers := make(map[int]bool, len(byzantinePosets))
	for _, creator := range byzantinePosets {
		forkers[creator] = true
	}

	random := rand.New(rand.NewSource(0))

	return newTriggeredAdder(
		func(unit gomel.Unit) bool {
			if forkers[unit.Creator()] && random.Intn(100) <= forkProbability {
				return true
			}
			return false
		},

		func(posets []*growing.Poset, unit gomel.Preunit) error {
			fmt.Println("random forking")
			const forkSize = 2
			for _, unit := range createForks(unit, privKeys[unit.Creator()], forkSize) {
				if _, err := offline_poset.AddToPosets(unit, posets); err != nil {
					return err
				}
			}
			fmt.Println("random forking finished")
			return nil
		},
	)
}

func testPrimeFloodingScenario() error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := offline_poset.GenerateKeys(nProcesses)

	unitCreator := offline_poset.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newPrimeFloodAdder(floodingLevel, forkingPrimes, privKeys, byzantinePosets)
	verifier := offline_poset.NewDefaultVerifier()
	testingRoutineFactory := offline_poset.NewDefaultTestingRoutineFactory(
		func(posets []*growing.Poset) []*growing.Poset { return posets },
		unitCreator,
		unitAdder,
		verifier,
	)

	return offline_poset.Test(pubKeys, nUnits, maxParents, testingRoutineFactory)
}

func testSimpleScenario() error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := offline_poset.GenerateKeys(nProcesses)

	unitCreator := offline_poset.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newSimpleForkingAdder(10, privKeys, byzantinePosets)
	verifier := offline_poset.NewDefaultVerifier()
	testingRoutineFactory := offline_poset.NewDefaultTestingRoutineFactory(
		func(posets []*growing.Poset) []*growing.Poset { return posets },
		unitCreator,
		unitAdder,
		verifier,
	)

	return offline_poset.Test(pubKeys, nUnits, maxParents, testingRoutineFactory)
}

func testRandomForking() error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := offline_poset.GenerateKeys(nProcesses)

	unitCreator := offline_poset.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newRandomForkingAdder(byzantinePosets, 50, privKeys)
	verifier := offline_poset.NewDefaultVerifier()
	testingRoutineFactory := offline_poset.NewDefaultTestingRoutineFactory(
		func(posets []*growing.Poset) []*growing.Poset { return posets },
		unitCreator,
		unitAdder,
		verifier,
	)

	return offline_poset.Test(pubKeys, nUnits, maxParents, testingRoutineFactory)
}

var _ = Describe("Byzantine Poset Test", func() {
	Describe("simple scenario", func() {
		It("should finish without errors", func() {
			err := testSimpleScenario()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("prime flooding scenario", func() {
		It("should finish without errors", func() {
			err := testPrimeFloodingScenario()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("random forking scenario", func() {
		It("should finish without errors", func() {
			err := testRandomForking()
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
