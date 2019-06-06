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
	"gitlab.com/alephledger/consensus-go/tests/offline_poset/helpers"
)

type forkingStrategy func(gomel.Preunit, gomel.Poset, gomel.PrivateKey, int) []gomel.Preunit

// Implementation of the Preunit interface that allows to create arbitrary forks without changing units content
type preunitWithNonce struct {
	gomel.Preunit
	nounce int32
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

func newPreunitWithNounce(preunit gomel.Preunit, nounce int32) *preunitWithNonce {
	pu := creating.NewPreunit(preunit.Creator(), preunit.Parents(), []byte{}, nil, nil)

	result := preunitWithNonce{Preunit: pu, nounce: nounce}
	result.computeHash()
	return &result
}

func (pu *preunitWithNonce) Hash() *gomel.Hash {
	return &pu.hash
}

func createForks(preunit gomel.Preunit, poset gomel.Poset, privKey gomel.PrivateKey, count int) []gomel.Preunit {
	result := make([]gomel.Preunit, 0, count)
	created := map[gomel.Hash]bool{*preunit.Hash(): true}
	for nounce := int32(0); len(result) < count; nounce++ {
		fork := newPreunitWithNounce(preunit, nounce)
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
	return (nProc - 1) / 3
}

func getRandomListOfByzantinePosets(n int) []int {
	byzProcesses := computeMaxPossibleNumberOfByzantineProcesses(n)
	return rand.Perm(byzProcesses)[:byzProcesses]
}

func newTriggeredAdder(triggerCondition func(unit gomel.Unit) bool, wrappedHandler helpers.AddingHandler) helpers.AddingHandler {

	return func(posets []gomel.Poset, unit gomel.Preunit) error {
		newUnit, err := helpers.AddToPosets(unit, posets)
		if err != nil {
			return err
		}
		if triggerCondition(newUnit) {
			return wrappedHandler(posets, unit)
		}
		return nil
	}
}

func newSimpleForkingAdder(forkingLevel int, privKeys []gomel.PrivateKey, byzantinePosets []int, forkingStrategy forkingStrategy) helpers.AddingHandler {
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

		func(posets []gomel.Poset, unit gomel.Preunit) error {
			fmt.Println("simple forking behavior triggered")
			units := forkingStrategy(unit, posets[unit.Creator()], privKeys[unit.Creator()], 2)
			if len(units) == 0 {
				return nil
			}
			err := helpers.AddUnitsToPosetsInRandomOrder(units, posets)
			if err != nil {
				return err
			}
			alreadyForked[unit.Creator()] = true
			fmt.Println("simple fork created at level", forkingLevel)
			return nil
		},
	)
}

func newPrimeFloodAdder(floodingLevel int, numberOfPrimes int, privKeys []gomel.PrivateKey, byzantinePosets []int, forkingStrategy forkingStrategy) helpers.AddingHandler {
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

		func(posets []gomel.Poset, unit gomel.Preunit) error {
			fmt.Println("Prime flooding started")
			for _, unit := range forkingStrategy(unit, posets[unit.Creator()], privKeys[unit.Creator()], numberOfPrimes) {
				if _, err := helpers.AddToPosets(unit, posets); err != nil {
					return err
				}
			}
			alreadyFlooded[unit.Creator()] = true
			fmt.Println("Prime flooding finished")
			return nil
		},
	)
}

func newRandomForkingAdder(byzantinePosets []int, forkProbability int, privKeys []gomel.PrivateKey, forkingStrategy forkingStrategy) helpers.AddingHandler {
	forkers := make(map[int]bool, len(byzantinePosets))
	for _, creator := range byzantinePosets {
		forkers[creator] = true
	}

	random := rand.New(rand.NewSource(7))

	return newTriggeredAdder(
		func(unit gomel.Unit) bool {
			if forkers[unit.Creator()] && random.Intn(100) <= forkProbability {
				return true
			}
			return false
		},

		func(posets []gomel.Poset, unit gomel.Preunit) error {
			fmt.Println("random forking")
			const forkSize = 2
			for _, unit := range forkingStrategy(unit, posets[unit.Creator()], privKeys[unit.Creator()], forkSize) {
				if _, err := helpers.AddToPosets(unit, posets); err != nil {
					return err
				}
			}
			fmt.Println("random forking finished")
			return nil
		},
	)
}

func testPrimeFloodingScenario(forkingStrategy forkingStrategy) error {
	const (
		nProcesses    = 21
		nUnits        = 1000
		maxParents    = 2
		forkingPrimes = 1000
		floodingLevel = 10
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newPrimeFloodAdder(floodingLevel, forkingPrimes, privKeys, byzantinePosets, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, nUnits, maxParents, testingRoutine)
}

func testSimpleForkingScenario(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newSimpleForkingAdder(10, privKeys, byzantinePosets, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, nUnits, maxParents, testingRoutine)
}

func testRandomForking(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(privKeys)
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newRandomForkingAdder(byzantinePosets, 50, privKeys, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, nUnits, maxParents, testingRoutine)
}

var _ = Describe("Byzantine Poset Test", func() {
	Describe("simple scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testSimpleForkingScenario(createForks)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("prime flooding scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testPrimeFloodingScenario(createForks)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("random forking scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testRandomForking(createForks)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
