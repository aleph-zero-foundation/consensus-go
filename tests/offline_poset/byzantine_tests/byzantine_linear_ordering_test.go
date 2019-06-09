package byzantine_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/tests/offline_poset/helpers"
)

type forkingStrategy func(gomel.Preunit, gomel.Poset, gomel.PrivateKey, int) []gomel.Preunit

type forker func(preunit gomel.Preunit, poset gomel.Poset, privKey gomel.PrivateKey) (gomel.Preunit, error)

func newForkWithDifferentData(preunit gomel.Preunit) gomel.Preunit {
	data := append(preunit.Data(), 1)
	return creating.NewPreunit(
		preunit.Creator(),
		preunit.Parents(),
		data,
		preunit.CoinShare(),
		preunit.ThresholdCoinData(),
	)
}

func newForkerUsingDifferentDataStrategy() forker {
	return func(preunit gomel.Preunit, poset gomel.Poset, privKey gomel.PrivateKey) (gomel.Preunit, error) {
		return newForkWithDifferentData(preunit), nil
	}
}

func createForkWithRandomParents(parentsCount int, rand *rand.Rand) forker {
	return func(preunit gomel.Preunit, poset gomel.Poset, privKey gomel.PrivateKey) (gomel.Preunit, error) {

		parents := make([]*gomel.Hash, 0, parentsCount)
		parentUnits := make([]gomel.Unit, 0, parentsCount)
		firstParent := preunit.Parents()[0]
		parents = append(parents, firstParent)
		parentUnits = append(parentUnits, poset.Get([]*gomel.Hash{firstParent})[0])

		selfPredecessor := poset.Get([]*gomel.Hash{firstParent})[0]

		for _, pid := range rand.Perm(poset.NProc()) {
			if len(parents) >= parentsCount {
				break
			}
			if pid == preunit.Creator() {
				continue
			}

			availableParents := poset.MaximalUnitsPerProcess().Get(pid)
			for len(availableParents) > 0 {
				randIx := rand.Intn(len(availableParents))
				selectedParent := availableParents[randIx]
				availableParents[len(availableParents)-1], availableParents[randIx] =
					availableParents[randIx], availableParents[len(availableParents)-1]
				parentUnits = append(parentUnits, selectedParent)
				if growing.CheckExpandPrimes(poset, parentUnits) != nil {
					parentUnits = parentUnits[:len(parentUnits)-1]
					predecessor, err := gomel.Predecessor(selectedParent)
					if err != nil || predecessor.Below(selfPredecessor) {
						availableParents = availableParents[:len(availableParents)-1]
					} else {
						availableParents[len(availableParents)-1] = predecessor
					}
					continue
				}
				parents = append(parents, selectedParent.Hash())
				availableParents = availableParents[:len(availableParents)-1]
			}
		}
		if len(parents) != parentsCount {
			return nil, errors.New("unable to collect enough parents")
		}
		return creating.NewPreunit(preunit.Creator(), parents, preunit.Data(), nil, nil), nil
	}
}

func createForksUsingForker(forker forker) forkingStrategy {

	return func(preunit gomel.Preunit, poset gomel.Poset, privKey gomel.PrivateKey, count int) []gomel.Preunit {

		created := make(map[gomel.Hash]bool, count)
		result := make([]gomel.Preunit, 0, count)

		for len(result) < count {
			fork, err := forker(preunit, poset, privKey)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				continue
			}
			hash := *fork.Hash()
			if created[hash] {
				continue
			}
			created[hash] = true
			fork.SetSignature(privKey.Sign(fork))
			result = append(result, fork)
			preunit = fork
		}

		return result
	}
}

func newDefaultUnitCreator(maxParents uint16) helpers.Creator {
	return func(poset gomel.Poset, creator uint16, privKey gomel.PrivateKey) (gomel.Preunit, error) {
		pu, err := creating.NewUnit(poset, int(creator), int(maxParents), helpers.NewDefaultDataContent())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
			return nil, err
		}
		pu.SetSignature(privKey.Sign(pu))
		return pu, nil
	}
}

func newForkAndHideAdder(
	createLevel, buildLevel, showOffLevel uint64,
	forker uint16,
	privKey gomel.PrivateKey,
	forkingStrategy forkingStrategy,
	numberOfForks int,
	maxParents uint16,
) (helpers.Creator, func(posets []gomel.Poset, preunit gomel.Preunit, unit gomel.Unit) error) {
	if createLevel > buildLevel {
		panic("'createLevel' should be not larger than 'buildLevel'")
	}
	if buildLevel > showOffLevel {
		panic("'buildLevel' should not be larger than 'showOffLevel'")
	}

	var createdForks []gomel.Preunit
	var forkingRoot gomel.Preunit
	var forkingRootUnit gomel.Unit
	alreadyAdded := false
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	switchCounter := 0

	defaultUnitCreator := newDefaultUnitCreator(maxParents)
	unitCreator := func(poset gomel.Poset, creator uint16, privKey gomel.PrivateKey) (gomel.Preunit, error) {
		// do not create new units after you showed a fork
		if alreadyAdded {
			return nil, nil
		}
		pu, err := defaultUnitCreator(poset, forker, privKey)
		if err != nil {
			return nil, err
		}
		return pu, nil
	}

	addingHandler := func(posets []gomel.Poset, preunit gomel.Preunit, unit gomel.Unit) error {
		if alreadyAdded {
			return nil
		}

		// remember our root for forking
		if forkingRoot == nil && uint16(preunit.Creator()) == forker && uint64(unit.Level()) >= createLevel {
			forkingRoot = preunit
			forkingRootUnit = unit
			switchCounter = 1
		}

		// randomly change forkingRoot if current unit is on the same level
		if forkingRootUnit != nil && forkingRootUnit.Level() == unit.Level() && uint16(preunit.Creator()) == forker {
			// if forkingRootUnit != nil && forkingRootUnit.Level() == unit.Level() && uint16(preunit.Creator()) == forker {
			switchCounter++
			if rand.Intn(switchCounter) == 0 {
				forkingRoot = preunit
				forkingRootUnit = unit
			}
		}

		// after some time try to create a unit
		if forkingRoot != nil && uint64(unit.Level()) >= buildLevel && createdForks == nil {
			createdForks = forkingStrategy(forkingRoot, posets[forker], privKey, numberOfForks)
		}

		// add forking units to all posets
		if len(createdForks) > 0 && uint64(unit.Level()) >= showOffLevel {
			// show all created forks to all participants
			if err := helpers.AddUnitsToPosetsInRandomOrder(createdForks, posets); err != nil {
				return err
			}
			fmt.Println("Byzantine node added a fork:", createdForks[0].Creator())
			alreadyAdded = true
		}
		return nil
	}
	return unitCreator, addingHandler
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
	alreadyForked := make(map[uint16]bool, len(byzantinePosets))
	for _, posetID := range byzantinePosets {
		alreadyForked[uint16(posetID)] = false
	}

	return newTriggeredAdder(
		func(unit gomel.Unit) bool {
			val, ok := alreadyForked[uint16(unit.Creator())]
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
			alreadyForked[uint16(unit.Creator())] = true
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
	unitCreator := helpers.NewDefaultUnitCreator(privKeys, newDefaultUnitCreator(maxParents))
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newPrimeFloodAdder(floodingLevel, forkingPrimes, privKeys, byzantinePosets, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, nUnits, maxParents, testingRoutine)
}

func testSimpleForkingScenario(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(privKeys, newDefaultUnitCreator(maxParents))
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newSimpleForkingAdder(10, privKeys, byzantinePosets, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, nUnits, maxParents, testingRoutine)
}

func testRandomForking(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(privKeys, newDefaultUnitCreator(maxParents))
	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	unitAdder := newRandomForkingAdder(byzantinePosets, 50, privKeys, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, nUnits, maxParents, testingRoutine)
}

func testForkingChangingParents() error {
	const (
		nProcesses      = 21
		nUnits          = 1000
		maxParents      = 2
		votingLevel     = 4
		createLevel     = 10
		buildLevel      = createLevel + 1
		showOffLevel    = buildLevel + 2
		numberOfForks   = 2
		numberOfParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	byzantinePosets := getRandomListOfByzantinePosets(nProcesses)
	fmt.Println("byzantine posets:", byzantinePosets)

	type byzAddingHandler func(posets []gomel.Poset, preunit gomel.Preunit, unit gomel.Unit) error
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	byzPosets := map[uint16]struct {
		byzCreator helpers.Creator
		byzAdder   byzAddingHandler
	}{}
	for _, byzPoset := range byzantinePosets {

		unitCreator, addingHandler := newForkAndHideAdder(
			createLevel, buildLevel, showOffLevel,

			uint16(byzPoset),
			privKeys[byzPoset],
			createForksUsingForker(createForkWithRandomParents(numberOfParents, rand)),
			numberOfForks,
			maxParents,
		)

		byzPosets[uint16(byzPoset)] = struct {
			byzCreator helpers.Creator
			byzAdder   byzAddingHandler
		}{unitCreator, addingHandler}
	}

	defaultUnitCreator := newDefaultUnitCreator(maxParents)
	unitFactory := func(poset gomel.Poset, creator uint16, privKey gomel.PrivateKey) (gomel.Preunit, error) {
		if byzPoset, ok := byzPosets[creator]; ok {
			return byzPoset.byzCreator(poset, creator, privKey)

		}
		return defaultUnitCreator(poset, creator, privKey)
	}
	unitCreator := helpers.NewDefaultUnitCreator(privKeys, unitFactory)

	unitAdder := func(posets []gomel.Poset, preunit gomel.Preunit) error {
		unit, err := helpers.AddToPosets(preunit, posets)
		if err != nil {
			return err
		}
		for _, byzPoset := range byzPosets {
			if err := byzPoset.byzAdder(posets, preunit, unit); err != nil {
				return err
			}
		}
		return nil
	}

	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		unitCreator,
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, nUnits, maxParents, testingRoutine)
}

var _ = Describe("Byzantine Poset Test", func() {
	Describe("simple scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testSimpleForkingScenario(createForksUsingForker(newForkerUsingDifferentDataStrategy()))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("prime flooding scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testPrimeFloodingScenario(createForksUsingForker(newForkerUsingDifferentDataStrategy()))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("random forking scenario", func() {
		Context("using same parents for forks", func() {
			It("should finish without errors", func() {
				err := testRandomForking(createForksUsingForker(newForkerUsingDifferentDataStrategy()))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("fork with different parents", func() {
		It("should finish without errors", func() {
			err := testForkingChangingParents()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
