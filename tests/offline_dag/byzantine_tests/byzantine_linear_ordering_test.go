package byzantine_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/consensus-go/tests/offline_dag/helpers"
)

type forkingStrategy func(gomel.Preunit, gomel.Dag, gomel.PrivateKey, gomel.RandomSource, int) []gomel.Preunit

type forker func(gomel.Preunit, gomel.Dag, gomel.PrivateKey, gomel.RandomSource) (gomel.Preunit, error)

func generateFreshData(preunitData []byte) []byte {
	var data []byte
	data = append(data, preunitData...)
	if len(data) > 0 && data[len(data)-1] < math.MaxUint8 {
		data[len(data)-1]++
	} else {
		data = append(data, 0)
	}
	return data
}

func newForkWithDifferentData(preunit gomel.Preunit) gomel.Preunit {
	data := generateFreshData(preunit.Data())
	return creating.NewPreunit(
		preunit.Creator(),
		preunit.EpochID(),
		preunit.View(),
		data,
		preunit.RandomSourceData(),
	)
}

func newForkerUsingDifferentDataStrategy() forker {
	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		return newForkWithDifferentData(preunit), nil
	}
}

func createForkUsingNewUnit() forker {
	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {

		pu, _, err := creating.NewUnit(dag, preunit.Creator(), helpers.NewDefaultDataContent(), rs, true)
		if err != nil {
			return nil, fmt.Errorf("unable to create a forking unit: %s", err.Error())
		}

		preunitParents, err := dag.DecodeParents(preunit)
		if err != nil {
			return nil, err
		}
		puParents, err := dag.DecodeParents(pu)
		if err != nil {
			return nil, err
		}

		puParents[pu.Creator()] = preunitParents[pu.Creator()]
		freshData := generateFreshData(preunit.Data())
		level := helpers.ComputeLevel(dag, puParents)
		rsData, err := rs.DataToInclude(pu.Creator(), puParents, level)
		if err != nil {
			return nil, err
		}

		return preunitFromParents(pu.Creator(), pu.EpochID(), puParents, freshData, rsData), nil
	}
}

func checkSelfForkingEvidence(parents []gomel.Unit, creator uint16) bool {
	if parents[creator] == nil {
		return false
	}
	// using the knowledge of maximal units produced by 'creator' that are below some of the parents (their floor attributes),
	// check whether collection of these maximal units has a single maximal element
	combinedFloor := gomel.MaximalByPid(parents, creator)
	return len(combinedFloor) > 1 || (len(combinedFloor) == 1 && !gomel.Equal(combinedFloor[0], parents[creator]))
}

func checkCompliance(dag gomel.Dag, creator uint16, parents []gomel.Unit) error {
	if checkSelfForkingEvidence(parents, creator) {
		return gomel.NewComplianceError("parents contain evidence of self forking")
	}
	if check.ForkerMutingCheck(parents) != nil {
		return gomel.NewComplianceError("parents do not satisfy the forker-muting rule")
	}
	return nil
}

func createForkWithRandomParents(parentsCount uint16, rand *rand.Rand) forker {

	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {

		preunitParents, err := dag.DecodeParents(preunit)
		if err != nil {
			return nil, err
		}
		parents := []*gomel.Hash{}
		parentUnits := []gomel.Unit{}

		selfPredecessor := preunitParents[preunit.Creator()]
		parents = append(parents, selfPredecessor.Hash())
		parentUnits = append(parentUnits, selfPredecessor)

		for _, pid := range rand.Perm(int(dag.NProc())) {
			if len(parents) >= int(parentsCount) {
				break
			}
			if pid == int(preunit.Creator()) {
				continue
			}

			var availableParents []gomel.Unit
			availableParents = append(availableParents, dag.MaximalUnitsPerProcess().Get(uint16(pid))...)

			for len(availableParents) > 0 {
				randIx := rand.Intn(len(availableParents))
				selectedParent := availableParents[randIx]
				availableParents[len(availableParents)-1], availableParents[randIx] =
					availableParents[randIx], availableParents[len(availableParents)-1]
				parentUnits = append(parentUnits, selectedParent)
				if err := checkCompliance(dag, preunit.Creator(), parentUnits); err != nil {
					parentUnits = parentUnits[:len(parentUnits)-1]
					predecessor := gomel.Predecessor(selectedParent)
					if predecessor == nil || gomel.Above(selfPredecessor, predecessor) {
						availableParents = availableParents[:len(availableParents)-1]
					} else {
						availableParents[len(availableParents)-1] = predecessor
					}
					continue
				}
				parents = append(parents, selectedParent.Hash())
				break
			}
		}
		if len(parents) < 2 {
			return nil, errors.New("unable to collect enough parents")
		}

		sortedParents := make([]gomel.Unit, dag.NProc())
		for _, p := range parentUnits {
			sortedParents[p.Creator()] = p
		}

		freshData := generateFreshData(preunit.Data())
		level := helpers.ComputeLevel(dag, sortedParents)
		rsData, err := rs.DataToInclude(preunit.Creator(), sortedParents, level)
		if err != nil {
			return nil, err
		}
		return preunitFromParents(preunit.Creator(), preunit.EpochID(), sortedParents, freshData, rsData), nil
	}
}

func createForksUsingForker(forker forker) forkingStrategy {

	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource, count int) []gomel.Preunit {

		created := make(map[gomel.Hash]bool, count)
		result := make([]gomel.Preunit, 0, count)

		for len(result) < count {
			fork, err := forker(preunit, dag, privKey, rs)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				continue
			}
			hash := *fork.Hash()
			if created[hash] {
				continue
			}
			fork.SetSignature(privKey.Sign(fork))
			created[hash] = true
			result = append(result, fork)
			preunit = fork
		}

		return result
	}
}

func newForkAndHideAdder(
	createLevel, buildLevel, showOffLevel int,
	forker uint16,
	privKey gomel.PrivateKey,
	forkingStrategy forkingStrategy,
	numberOfForks int,
	maxParents uint16,
) (helpers.Creator, func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit, unit gomel.Unit) error, error) {
	if createLevel > buildLevel {
		return nil, nil, errors.New("'createLevel' should be not larger than 'buildLevel'")
	}
	if buildLevel > showOffLevel {
		return nil, nil, errors.New("'buildLevel' should not be larger than 'showOffLevel'")
	}

	var createdForks []gomel.Preunit
	var forkingRoot gomel.Preunit
	var forkingRootUnit gomel.Unit
	alreadyAdded := false
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	switchCounter := 0

	defaultUnitCreator := helpers.NewDefaultCreator(maxParents)
	unitCreator := func(dag gomel.Dag, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		// do not create new units after we showed some fork
		if alreadyAdded {
			return nil, nil
		}
		pu, err := defaultUnitCreator(dag, forker, privKey, rs)
		if err != nil {
			return nil, err
		}
		return pu, nil
	}

	addingHandler := func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit, unit gomel.Unit) error {
		if alreadyAdded {
			return nil
		}

		// remember some unit which we are later going to use for forking
		if forkingRoot == nil && preunit.Creator() == forker && unit.Level() >= createLevel {
			forkingRoot = preunit
			forkingRootUnit = unit
			switchCounter = 1
		}

		// randomly change forkingRoot if current unit is on the same level
		if forkingRootUnit != nil && forkingRootUnit.Level() == unit.Level() && preunit.Creator() == forker {
			switchCounter++
			if rand.Intn(switchCounter) == 0 {
				forkingRoot = preunit
				forkingRootUnit = unit
			}
		}

		// after some time try to create a unit
		if forkingRoot != nil && unit.Level() >= buildLevel && createdForks == nil && unit.Creator() != forker {
			createdForks = forkingStrategy(forkingRoot, dags[forker], privKey, rss[forker], numberOfForks)
		}

		// add forking units to all dags
		if len(createdForks) > 0 && unit.Level() >= showOffLevel {
			// show all created forks to all participants
			if err := helpers.AddUnitsToDagsInRandomOrder(createdForks, dags); err != nil {
				return err
			}
			fmt.Println("Byzantine node added a fork:", createdForks[0].Creator())
			alreadyAdded = true
		}
		return nil
	}
	return unitCreator, addingHandler, nil
}

func computeMaxPossibleNumberOfByzantineProcesses(nProc uint16) uint16 {
	return (nProc - 1) / 3
}

func getRandomListOfByzantineDags(n uint16) []uint16 {
	byzProcesses := computeMaxPossibleNumberOfByzantineProcesses(n)
	perm := rand.Perm(int(byzProcesses))[:byzProcesses]
	result := make([]uint16, byzProcesses)
	for i, pid := range perm {
		result[i] = uint16(pid)
	}
	return result
}

func newTriggeredAdder(triggerCondition func(unit gomel.Unit) bool, wrappedHandler helpers.AddingHandler) helpers.AddingHandler {

	return func(dags []gomel.Dag, rss []gomel.RandomSource, unit gomel.Preunit) error {
		newUnit, err := helpers.AddToDags(unit, rss, dags)
		if err != nil {
			return err
		}
		if triggerCondition(newUnit) {
			return wrappedHandler(dags, rss, unit)
		}
		return nil
	}
}

func newSimpleForkingAdder(forkingLevel int, privKeys []gomel.PrivateKey, byzantineDags []uint16, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

	return func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {
		alreadyForked := make(map[uint16]bool, len(byzantineDags))
		for _, dagID := range byzantineDags {
			alreadyForked[uint16(dagID)] = false
		}
		allExecuted := false
		forkedProcesses := 0
		return newTriggeredAdder(
			func(unit gomel.Unit) bool {
				if allExecuted {
					return false
				}
				val, ok := alreadyForked[uint16(unit.Creator())]
				if ok && !val && unit.Level() >= forkingLevel {
					return true
				}
				return false
			},

			func(dags []gomel.Dag, rss []gomel.RandomSource, unit gomel.Preunit) error {
				fmt.Println("simple forking behavior triggered")
				units := forkingStrategy(unit, dags[unit.Creator()], privKeys[unit.Creator()], rss[unit.Creator()], 2)
				if len(units) == 0 {
					return nil
				}
				err := helpers.AddUnitsToDagsInRandomOrder(units, dags)
				if err != nil {
					return err
				}
				alreadyForked[uint16(unit.Creator())] = true
				forkedProcesses++
				if forkedProcesses == len(byzantineDags) {
					allExecuted = true
				}
				fmt.Println("simple fork created at level", forkingLevel)
				return nil
			},
		)
	}
}

func newPrimeFloodingAdder(floodingLevel int, numberOfPrimes int, privKeys []gomel.PrivateKey, byzantineDags []uint16, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

	return func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) helpers.AddingHandler {
		alreadyFlooded := make(map[uint16]bool, len(byzantineDags))
		for _, dagID := range byzantineDags {
			alreadyFlooded[dagID] = false
		}
		allExecuted := false
		forkedProcesses := 0
		return newTriggeredAdder(
			func(unit gomel.Unit) bool {
				if allExecuted {
					return false
				}
				val, ok := alreadyFlooded[unit.Creator()]
				if ok && !val && unit.Level() >= floodingLevel && gomel.Prime(unit) {
					return true
				}
				return false
			},

			func(dags []gomel.Dag, rss []gomel.RandomSource, unit gomel.Preunit) error {
				fmt.Println("Prime flooding started")
				for _, unit := range forkingStrategy(unit, dags[unit.Creator()], privKeys[unit.Creator()], rss[unit.Creator()], numberOfPrimes) {
					if _, err := helpers.AddToDags(unit, rss, dags); err != nil {
						return err
					}
				}
				alreadyFlooded[unit.Creator()] = true
				forkedProcesses++
				if forkedProcesses == len(byzantineDags) {
					allExecuted = true
				}
				fmt.Println("Prime flooding finished")
				return nil
			},
		)
	}
}

func newRandomForkingAdder(byzantineDags []uint16, forkProbability int, privKeys []gomel.PrivateKey, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

	return func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {
		forkers := make(map[uint16]bool, len(byzantineDags))
		for _, creator := range byzantineDags {
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

			func(dags []gomel.Dag, rss []gomel.RandomSource, unit gomel.Preunit) error {
				fmt.Println("random forking")
				const forkSize = 2
				for _, unit := range forkingStrategy(unit, dags[unit.Creator()], privKeys[unit.Creator()], rss[unit.Creator()], forkSize) {
					if _, err := helpers.AddToDags(unit, rss, dags); err != nil {
						return err
					}
				}
				fmt.Println("random forking finished")
				return nil
			},
		)
	}
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
	unitCreator := helpers.NewDefaultUnitCreator(helpers.NewDefaultCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newPrimeFloodingAdder(floodingLevel, forkingPrimes, privKeys, byzantineDags, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, helpers.NewDefaultConfigurations(nProcesses), testingRoutine)
}

func testSimpleForkingScenario(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)
	unitCreator := helpers.NewDefaultUnitCreator(helpers.NewDefaultCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newSimpleForkingAdder(10, privKeys, byzantineDags, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, helpers.NewDefaultConfigurations(nProcesses), testingRoutine)
}

func testRandomForking(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(helpers.NewDefaultCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newRandomForkingAdder(byzantineDags, 50, privKeys, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, helpers.NewDefaultConfigurations(nProcesses), testingRoutine)
}

func testForkingChangingParents(forker forker) error {
	const (
		nProcesses      = 21
		maxParents      = 2
		votingLevel     = 4
		createLevel     = 10
		buildLevel      = createLevel + 4
		showOffLevel    = buildLevel + 2
		numberOfForks   = 2
		numberOfParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	fmt.Println("byzantine dags:", byzantineDags)

	type byzAddingHandler func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit, unit gomel.Unit) error
	type byzPair struct {
		byzCreator helpers.Creator
		byzAdder   byzAddingHandler
	}
	byzDags := map[uint16]byzPair{}
	for _, byzDag := range byzantineDags {

		unitCreator, addingHandler, err := newForkAndHideAdder(
			createLevel, buildLevel, showOffLevel,

			byzDag,
			privKeys[byzDag],
			createForksUsingForker(forker),
			numberOfForks,
			maxParents,
		)

		if err != nil {
			return err
		}

		byzDags[byzDag] = byzPair{unitCreator, addingHandler}
	}

	defaultUnitCreator := helpers.NewDefaultCreator(maxParents)
	unitFactory := func(dag gomel.Dag, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		if byzDag, ok := byzDags[creator]; ok {
			return byzDag.byzCreator(dag, creator, privKey, rs)

		}
		return defaultUnitCreator(dag, creator, privKey, rs)
	}
	unitCreator := helpers.NewDefaultUnitCreator(unitFactory)

	unitAdder := func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {
		return func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {
			unit, err := helpers.AddToDags(preunit, rss, dags)
			if err != nil {
				return err
			}
			for _, byzDag := range byzDags {
				if err := byzDag.byzAdder(dags, rss, preunit, unit); err != nil {
					return err
				}
			}
			return nil
		}
	}

	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, helpers.NewDefaultConfigurations(nProcesses), testingRoutine)
}

func fixCommonVotes(commonVotes <-chan bool, initialVotingRound int) <-chan bool {
	fixedVotes := make(chan bool)
	go func() {
		for it := 0; it < initialVotingRound; it++ {
			fixedVotes <- true
		}
		for vote := range commonVotes {
			fixedVotes <- vote
		}
		close(fixedVotes)
	}()
	return fixedVotes
}

func newDefaultCommonVote(uc gomel.Unit, initialVotingRound int, lastDeterministicRound int) <-chan bool {
	commonVotes := make(chan bool)
	const deterministicPrefix = 10
	go func() {
		commonVotes <- true
		commonVotes <- false
		for round := 4; round <= deterministicPrefix; round++ {
			commonVotes <- true
		}

		// use the simplecoin to predict future common votes
		lastLevel := uc.Level() + int(lastDeterministicRound)
		for level := uc.Level() + int(deterministicPrefix) + 1; level <= lastLevel; level++ {
			commonVotes <- helpers.SimpleCoin(uc.Creator(), level+1)
		}
		close(commonVotes)
	}()
	return commonVotes
}

func syncDags(dag1, dag2 gomel.Dag, rs1, rs2 gomel.RandomSource) (bool, error) {
	dag1Max := dag1.MaximalUnitsPerProcess()
	dag2Max := dag2.MaximalUnitsPerProcess()
	missingForDag2 := map[gomel.Unit]bool{}
	missingForDag1 := map[gomel.Unit]bool{}
	for pid := uint16(0); pid < dag1.NProc(); pid++ {
		dag1Units := append([]gomel.Unit(nil), dag1Max.Get(pid)...)
		dag2Units := append([]gomel.Unit(nil), dag2Max.Get(pid)...)

		different := func(units []gomel.Unit, dag gomel.Dag) map[gomel.Unit]bool {
			missing := map[gomel.Unit]bool{}
			for _, unit := range units {
				// descend to a common parent
				current := unit
				for current != nil {
					other := dag.GetUnits([]*gomel.Hash{current.Hash()})
					if len(other) < 1 || other[0] == nil {
						if missing[current] {
							break
						} else {
							missing[current] = true
						}
					} else {
						break
					}

					predecessor := gomel.Predecessor(current)
					if predecessor == nil {
						current = nil
					} else {
						current = predecessor
					}
				}
			}
			return missing
		}
		for unit := range different(dag1Units, dag2) {
			if missingForDag2[unit] {
				break
			}
			missingForDag2[unit] = true
		}
		for unit := range different(dag2Units, dag1) {
			if missingForDag1[unit] {
				break
			}
			missingForDag1[unit] = true
		}
	}

	// sort units topologically
	missingForDag1Slice := topoSort(missingForDag1)
	missingForDag2Slice := topoSort(missingForDag2)

	adder := func(units []gomel.Unit, dag gomel.Dag, rs gomel.RandomSource) error {
		for _, unit := range units {
			preunit := unitToPreunit(unit)
			_, err := tests.AddUnit(dag, preunit)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if err := adder(missingForDag1Slice, dag1, rs1); err != nil {
		return true, err
	}
	if err := adder(missingForDag2Slice, dag2, rs2); err != nil {
		return true, err
	}
	return len(missingForDag1) > 0 || len(missingForDag2) > 0, nil
}

func topoSort(units map[gomel.Unit]bool) []gomel.Unit {
	result := make([]gomel.Unit, 0, len(units))
	return buildReverseDfsOrder(units, result)
}

func buildReverseDfsOrder(units map[gomel.Unit]bool, result []gomel.Unit) []gomel.Unit {
	notVisited := map[gomel.Unit]bool{}
	for unit := range units {
		notVisited[unit] = true
	}
	for unit := range units {
		result = reverseDfsOrder(unit, notVisited, result)
	}
	return result
}

func reverseDfsOrder(unit gomel.Unit, notVisited map[gomel.Unit]bool, result []gomel.Unit) []gomel.Unit {
	if notVisited[unit] {
		notVisited[unit] = false
		for _, parent := range unit.Parents() {
			result = reverseDfsOrder(parent, notVisited, result)
		}
		result = append(result, unit)
	}
	return result
}

func syncAllDags(dags []gomel.Dag, rss []gomel.RandomSource) error {
	if len(dags) < 2 {
		return nil
	}
	for different := true; different; {
		different = false
		first := dags[0]
		firstRss := rss[0]
		for ix, dag := range dags[1:] {
			diff, err := syncDags(first, dag, firstRss, rss[ix+1])
			if err != nil {
				return err
			}
			if diff {
				different = true
			}
		}
	}
	return nil
}

func countUnitsOnLevelOrHigher(dag gomel.Dag, level int) map[uint16]bool {
	seen := make(map[uint16]bool, dag.NProc())
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, unit := range units {
			if unit.Level() >= level {
				seen[unit.Creator()] = true
				if dag.IsQuorum(uint16(len(seen))) {
					return false
				}
			}
		}
		return true
	})
	return seen
}

// Description of the algorithm:
// In order to 'cheat' the decision procedure we maintain the following invariant: if the common vote for the next round
// equals v, then during the current round there must be at most f units voting for v (or equivalently there is N-f units voting
// for the value 1-v). The following algorithm attempts to maintain this invariant for all of the provided values of the common
// vote.

// High-level description (without initialization):

// Maintain two independent 'towers' (subsets processes), first consisting of process voting for `1` and the other voting for
// `0`. Depending on the value of the next common vote, their sizes are 'f' and 'N-f' respectively. For example, when the value
// of the next common vote equals 1, then 1's tower consist of 'f' processes and the 0's tower of 'N-f'. This way the decision
// procedure is unable to commit its value - common vote 1 is different from the value that was selected by the side having
// super-majority of equal votes (other side votes 'undecided' and so chooses the common vote as its value). For simplicity,
// lets assume we are currently building units for one of the towers consisting of 'N-f' processes. We keep building new levels
// on the same side until the value of the next common vote forces us to switch sides, i.e. during the last round we created
// only 'f' prime units and so we are unable to create a new level without looking at units from the other tower. While building
// consecutive levels we enqueue all of the created units for the opposite side, so after current side finishes or switches, the
// other one can reveal them one by one till it is able to create a unit of its succeeding level. We also caches all common
// votes that we read, since the other side will not be able to read them from the provided channel after we processed them.

// The initialization process, which is all levels before the initial voting, is little bit tricky. We need to ensure that a
// unit U_c for which we are deciding becomes popular before the initial voting starts, but also allow at most 2f processes to
// record that it is popular on the voting level. This way it will not be decided 0 (as well as 1) at the round no
// initialVoting+1, since we made it popular, neither it will be decided 1 because we did not show it to enough processes. We
// achieve this goal by two means: extending common votes by some initial values and reverting the list of processes that vote 1
// on level initialVoting-1. Former allows us to treat the initialization similar way as any other round. For details, see the
// `fixCommonVotes` function. Later, makes the unit U_c not being decided 1 by the 'fast' algorithm. To this point, the extended
// common vote (0 for round `initialVoting`) forces us to make the unit U_c popular on level `initialVoting-1` (subset of
// processes voting 1 being of size 2f+1). If we would not reverse processes on the 1's tower list, then there would be a chance
// that processes on the zeros side would construct a proof of popularity of U_c, i.e. f+1 nodes that are shared between 1's and
// 0's from round `initialVoting-1` would introduce, having them as parents, f new nodes that are above U_c from ones side which
// would give us a quorum of processes that see U_c. After we reverse the 1's side, the 0's side uses f shared nodes from round
// initialVoting-1 that are not able to introduce any new processes (till that round they were building their levels alone using
// nodes from 0's side) and one additional processes that can only introduce f processes above U_c that we already know, giving
// us f+1 processes above U_c in total.

// tower:                  1 (votes 1) tower        0 tower
//
//                  1 |        ? (d+)               ? (v+)
//                    |
// (initial voting) 1 |        f (d)                2f+1**(v)
//                    |
// common vote      0 |        f (v)                2f+1 (d)
//                    |
//                  1 |        2f+1                 f
//                 -------------------------------------------
//                                      process

// ** to build new level we only use units from this tower (previous round consists of 2f+1 processes which is a quorum)
// + type of vote, i.e. d is voting using the value of the common vote, v is voting using supermajority
func makeDagsUndecidedForLongTime(
	dags []gomel.Dag,
	privateKeys []gomel.PrivateKey,
	ids []uint16,
	rss []gomel.RandomSource,
	decisionPreunit gomel.Preunit,
	decisionUnit gomel.Unit,
	commonVotes <-chan bool,
	initialVotingRound int,
) error {
	// ASSUMPTIONS:
	// 1) decisionUnit is already added to its owner's dag but every other dag is at the same level or lower
	// 2) author of the decisionUnit is first on the list of dags
	fmt.Println("decision level", decisionUnit.Level())

	commonVotes = fixCommonVotes(commonVotes, initialVotingRound)

	// left side are 'ones' (processes voting for 1) and right are 'zeros'

	var leftVotes []bool
	var rightVotes []bool
	var myVotes, otherVotes *[]bool

	type pair struct {
		l gomel.Preunit
		r int
	}

	var awaitingUnitsForLeft, awaitingUnitsForRight []pair
	var leftUnits, rightUnits []pair
	var awaitingUnits, unitsForOtherSide, myUnits *[]pair
	var leftKeys, rightKeys []gomel.PrivateKey
	var leftIds, rightIds []uint16
	var leftSide, rightSide []gomel.Dag
	var leftRss, rightRss []gomel.RandomSource

	// aim for 0 (2f+1 on the side of 0's)
	ones, zeros := subsetSizesOfLowerLevelBasedOnCommonVote(true, uint16(len(dags)))
	leftSide = dags[:ones]
	rightSide = dags[ones:]
	leftKeys = privateKeys[:ones]
	rightKeys = privateKeys[ones:]
	leftIds = ids[:ones]
	rightIds = ids[ones:]
	leftRss = rss[:ones]
	rightRss = rss[ones:]

	var currentSet *[]gomel.Dag
	var currentKeys *[]gomel.PrivateKey
	var currentIds *[]uint16
	var currentRss *[]gomel.RandomSource
	level := decisionUnit.Level() - 1
	leftLevel, rightLevel := level, level
	var nextVote, isLeft bool

	switchSides := func() {

		if isLeft {
			isLeft = false
			currentSet = &rightSide
			currentKeys = &rightKeys
			currentIds = &rightIds
			currentRss = &rightRss

			leftLevel = level
			level = rightLevel
			awaitingUnits = &awaitingUnitsForRight
			unitsForOtherSide = &awaitingUnitsForLeft
			myUnits = &rightUnits
			myVotes = &rightVotes
			otherVotes = &leftVotes
		} else {
			isLeft = true
			currentSet = &leftSide
			currentKeys = &leftKeys
			currentIds = &leftIds
			currentRss = &leftRss

			rightLevel = level
			level = leftLevel
			awaitingUnits = &awaitingUnitsForLeft
			unitsForOtherSide = &awaitingUnitsForRight
			myUnits = &leftUnits
			myVotes = &leftVotes
			otherVotes = &rightVotes
		}
	}

	checkMissing := func(level int, seen map[uint16]bool, source []pair) map[uint16]bool {

		if dags[0].IsQuorum(uint16(len(seen))) {
			return seen
		}

		for _, newUnit := range source {

			if newUnit.r > level {
				break
			}

			if newUnit.r == level {
				seen[newUnit.l.Creator()] = true
				if dags[0].IsQuorum(uint16(len(seen))) {
					return seen
				}
			}
		}

		return seen
	}

	addMissing := func(level int, seen map[uint16]bool, source *[]pair, sink []gomel.Dag, sinkRss []gomel.RandomSource) map[uint16]bool {

		for !dags[0].IsQuorum(uint16(len(seen))) && len(*source) > 0 {

			newUnit := (*source)[0]
			if newUnit.r > level {
				break
			}
			addedUnit := helpers.AddToDagsIngoringErrors(newUnit.l, sink)
			*source = (*source)[1:]
			if addedUnit != nil && addedUnit.Level() == level {
				seen[uint16(addedUnit.Creator())] = true
			}
		}

		return seen
	}

	isLeft = true
	switchSides()

	leftSide = dags[:zeros]
	leftKeys = privateKeys[:zeros]
	leftIds = ids[:zeros]
	leftRss = rss[:zeros]

	// initialize using units from the previous level
	dags[0].MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, unit := range units {
			if unit.Level() >= level {
				preunit := unitToPreunit(unit)
				leftUnits = append(leftUnits, pair{preunit, level})
				rightUnits = append(rightUnits, pair{preunit, level})
				break
			}
		}
		return true
	})

	// main loop that tries to grow two 'towers', one targeting 1 and the other 0 (1 means "I vote 1 for the 'decisionUnit'")
	for {
		// use minimal amount of units from the other side to build up by a new level

		seen := map[uint16]bool{}
		seen = checkMissing(level, seen, *myUnits)
		if !dags[0].IsQuorum(uint16(len(seen))) {
			seen = checkMissing(level, seen, *awaitingUnits)
		}
		if !dags[0].IsQuorum(uint16(len(seen))) {
			// we are definitely unable to build new level
			switchSides()
			continue
		}

		// at this point we know that we are able to create a new level

		// get the next vote from the channel or the list of already processed votes
		// if both are empty, then gracefully finish
		if len(*myVotes) > 0 {
			nextVote = (*myVotes)[0]
			*myVotes = (*myVotes)[1:]
		} else {
			var ok bool
			nextVote, ok = <-commonVotes
			if ok {
				*otherVotes = append(*otherVotes, nextVote)
			} else {
				if len(*otherVotes) > 0 {
					switchSides()
					continue
				} else {
					// nothing left to do here
					// just add all remaining units
					err := syncAllDags(dags, rss)
					if err != nil {
						return err
					}
					addMissing(level+1, map[uint16]bool{}, &leftUnits, dags, rss)
					addMissing(level+1, map[uint16]bool{}, &rightUnits, dags, rss)
					addMissing(level+1, map[uint16]bool{}, &awaitingUnitsForLeft, dags, rss)
					addMissing(level+1, map[uint16]bool{}, &awaitingUnitsForRight, dags, rss)
					break
				}
			}
		}

		// promote all required processes to the next level
		ones, _ := subsetSizesOfLowerLevelBasedOnCommonVote(nextVote, uint16(len(dags)))
		if isLeft {
			leftSide = dags[:ones]
			leftKeys = privateKeys[:ones]
			leftIds = ids[:ones]
			leftRss = rss[:ones]
		} else {
			rightSide = dags[ones:]
			rightKeys = privateKeys[ones:]
			rightIds = ids[ones:]
			rightRss = rss[ones:]
		}

		// synchronize all dags for the current round
		err := syncAllDags(*currentSet, *currentRss)
		if err != nil {
			return err
		}

		seen = countUnitsOnLevelOrHigher((*currentSet)[0], level)
		// show minimal number of units that allows us to build new level
		seen = addMissing(level, seen, myUnits, *currentSet, *currentRss)
		if !dags[0].IsQuorum(uint16(len(seen))) {
			seen = addMissing(level, seen, awaitingUnits, *currentSet, *currentRss)
		}

		// create new level
		createdUnits, err := buildOneLevelUp(*currentSet, *currentKeys, *currentIds, *currentRss, level+1)
		if err != nil {
			return err
		}
		level++
		fmt.Println("built a new level", level)

		// add newly created units to both queues (both sides)
		for _, unit := range createdUnits {
			*myUnits = append(*myUnits, pair{unit, level})
			*unitsForOtherSide = append(*unitsForOtherSide, pair{unit, level})
		}

		// special case used before initial voting
		if level+1 == decisionUnit.Level()+initialVotingRound {
			// reverse 1's, so they are not showing that U_c is popular to 0's
			for left, right := 0, len(*currentSet)-1; left < right; left, right = left+1, right-1 {
				(*currentSet)[left], (*currentSet)[right] = (*currentSet)[right], (*currentSet)[left]
			}
			for left, right := 0, len(*currentIds)-1; left < right; left, right = left+1, right-1 {
				(*currentIds)[left], (*currentIds)[right] = (*currentIds)[right], (*currentIds)[left]
			}
			for left, right := 0, len(*currentKeys)-1; left < right; left, right = left+1, right-1 {
				(*currentKeys)[left], (*currentKeys)[right] = (*currentKeys)[right], (*currentKeys)[left]
			}
			for left, right := 0, len(*currentRss)-1; left < right; left, right = left+1, right-1 {
				(*currentRss)[left], (*currentRss)[right] = (*currentRss)[right], (*currentRss)[left]
			}
		}
	}

	return nil
}

func subsetSizesOfLowerLevelBasedOnCommonVote(vote bool, nProc uint16) (ones, zeros uint16) {
	var maxNumberOfByzantineProcesses = computeMaxPossibleNumberOfByzantineProcesses(nProc)
	var nonByzantineProcesses = nProc - maxNumberOfByzantineProcesses
	if vote {
		return maxNumberOfByzantineProcesses, nonByzantineProcesses
	}
	return nonByzantineProcesses, maxNumberOfByzantineProcesses
}

func preunitFromParents(creator uint16, epochID gomel.EpochID, parents []gomel.Unit, data []byte, rsData []byte) gomel.Preunit {
	nProc := len(parents)
	heights := make([]int, nProc)
	hashes := make([]*gomel.Hash, nProc)
	for i, p := range parents {
		if p == nil {
			heights[i] = -1
			hashes[i] = &gomel.ZeroHash
		} else {
			heights[i] = p.Height()
			hashes[i] = p.Hash()
		}
	}
	return creating.NewPreunit(creator, epochID, gomel.NewCrown(heights, gomel.CombineHashes(hashes)), data, rsData)
}

func unitToPreunit(unit gomel.Unit) gomel.Preunit {
	return preunitFromParents(unit.Creator(), unit.EpochID(), unit.Parents(), unit.Data(), unit.RandomSourceData())
}

// this function assumes that it is possible to create a new level, i.e. there are enough candidates on lower level
func buildOneLevelUp(
	dags []gomel.Dag,
	privKeys []gomel.PrivateKey,
	ids []uint16,
	rss []gomel.RandomSource,
	level int,
) ([]gomel.Preunit, error) {
	// IMPORTANT instead of adding units immediately to all dags, save them so they will be processed on the next round

	createdUnits := make([]gomel.Preunit, 0, len(dags))
	for ix, dag := range dags {
		createdOnLevel := false
		for !createdOnLevel {
			// check if process/dag is already on that level
			for _, unit := range dag.MaximalUnitsPerProcess().Get(ids[ix]) {
				if unit.Level() == level {
					preunit := unitToPreunit(unit)
					createdUnits = append(createdUnits, preunit)
					createdOnLevel = true
					break
				}
			}
			if createdOnLevel {
				break
			}
			preunit, _, err := creating.NewUnit(dag, ids[ix], helpers.NewDefaultDataContent(), rss[ix], true)
			if err != nil {
				return nil, fmt.Errorf("error while creating a unit for dag no %d: %s", ids[ix], err.Error())
			}
			// add only to its creator's dag
			addedUnit, err := tests.AddUnit(dag, preunit)
			if err != nil {
				return nil, fmt.Errorf("error while adding to dag no %d: %s", ids[ix], err.Error())
			}

			createdUnits = append(createdUnits, preunit)
			if addedUnit.Level() == level {
				createdOnLevel = true
			}
		}
	}
	return createdUnits, nil
}

func longTimeUndecidedStrategy(startLevel *int, initialVotingRound int, numberOfDeterministicRounds int, crp func(level int) uint16) (func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler, func([]gomel.Dag) bool) {
	alreadyTriggered := false
	var lastCreated gomel.Unit
	resultAdder := func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) helpers.AddingHandler {
		seen := make(map[uint16]bool, len(dags))
		triggerCondition := func(unit gomel.Unit) bool {
			if alreadyTriggered {
				return false
			}
			if unit.Level() == (*startLevel - 1) {
				seen[unit.Creator()] = true
			}
			if dags[0].IsQuorum(uint16(len(seen))) {
				if unit.Creator() != crp(*startLevel) {
					lastCreated = unit
					return true
				}
				seen = map[uint16]bool{}
				*startLevel++
			}
			return false
		}

		addingHandler := func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {

			dagsCopy := append([]gomel.Dag{}, dags...)
			privKeysCopy := append([]gomel.PrivateKey{}, privKeys...)
			ids := make([]uint16, len(dags))
			rssCopy := append([]gomel.RandomSource{}, rss...)
			for ix := range dags {
				ids[ix] = uint16(ix)
			}

			triggeringDag := crp(*startLevel)
			fmt.Println("triggering dag no", triggeringDag)
			triggeringPreunit, _, err := creating.NewUnit(
				dags[triggeringDag],
				triggeringDag,
				helpers.NewDefaultDataContent(),
				rss[triggeringDag], true,
			)
			if err != nil {
				return err
			}
			triggeringUnit, err := tests.AddUnit(dags[triggeringDag], triggeringPreunit)
			if err != nil {
				return err
			}

			commonVotes := newDefaultCommonVote(triggeringUnit, initialVotingRound, numberOfDeterministicRounds)

			dagsCopy[triggeringDag], dagsCopy[0] = dagsCopy[0], dagsCopy[triggeringDag]
			privKeysCopy[triggeringDag], privKeysCopy[0] = privKeysCopy[0], privKeysCopy[triggeringDag]
			ids[triggeringDag], ids[0] = ids[0], ids[triggeringDag]
			rssCopy[triggeringDag], rssCopy[0] = rssCopy[0], rssCopy[triggeringDag]

			// move the last creator to the left side, so it will observer some new units before we ask it to create a new one
			dagsCopy[lastCreated.Creator()], dagsCopy[1] = dagsCopy[1], dagsCopy[lastCreated.Creator()]
			privKeysCopy[lastCreated.Creator()], privKeysCopy[1] = privKeysCopy[1], privKeysCopy[lastCreated.Creator()]
			ids[lastCreated.Creator()], ids[1] = ids[1], ids[lastCreated.Creator()]
			rssCopy[lastCreated.Creator()], rssCopy[1] = rssCopy[1], rssCopy[lastCreated.Creator()]

			result := makeDagsUndecidedForLongTime(dagsCopy, privKeysCopy, ids, rssCopy, triggeringPreunit, triggeringUnit, commonVotes, initialVotingRound)
			if result != nil {
				return result
			}
			alreadyTriggered = true
			return nil
		}
		return newTriggeredAdder(triggerCondition, addingHandler)
	}
	return resultAdder, func([]gomel.Dag) bool { return alreadyTriggered }
}

func testLongTimeUndecidedStrategy() error {
	const (
		nProcesses                  = 21
		nUnits                      = 1000
		maxParents                  = nProcesses
		numberOfDeterministicRounds = 50
		initialVotingRound          = 1
	)

	conf := config.NewDefaultConfiguration()

	// NOTE following 4 lines are supposed to enforce a unit of creator 1 being first on crp for level 1
	unitCreator := helpers.NewEachInSequenceUnitCreator(helpers.NewDefaultCreator(nProcesses))
	conf.CRPFixedPrefix = 1
	startLevel := 1
	crp := func(int) uint16 { return 1 }

	configurations := make([]config.Configuration, nProcesses)
	for pid := range configurations {
		configurations[pid] = conf
	}

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitAdder, stopCondition :=
		longTimeUndecidedStrategy(&startLevel, initialVotingRound, numberOfDeterministicRounds, crp)

	checkIfUndecidedVerifier :=
		func(dags []gomel.Dag, pids []uint16, configs []config.Configuration, rss []gomel.RandomSource) error {
			fmt.Println("starting the undecided checker")

			logger, _ := logging.NewLogger("stdout", conf.LogLevel, 100000, false)

			errorsCount := 0
			for pid, dag := range dags {
				ordering := linear.NewOrdering(
					dag,
					rss[pid],
					conf.OrderStartLevel,
					conf.CRPFixedPrefix,
					logger,
				)

				for tu := ordering.NextRound(); tu != nil && tu.TimingUnit().Level() < int(startLevel)-1; {
					tu = ordering.NextRound()
				}
				if timingRound := ordering.NextRound(); timingRound != nil && timingRound.TimingUnit().Level() >= startLevel {
					timingUnit := timingRound.TimingUnit()
					fmt.Println("some dag already decided - error", "level:", timingUnit.Level(), "creator:", timingUnit.Creator())
					errorsCount++
				}
			}
			if errorsCount > 0 {
				fmt.Println("number of errors", errorsCount)
				return fmt.Errorf("dags were suppose to not decide on this level - number of errors %d", errorsCount)
			}
			fmt.Println("dags were unable to make a decision after", numberOfDeterministicRounds, "rounds")
			return nil
		}

	testingRoutine := helpers.NewTestingRoutineWithStopCondition(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		func([]gomel.Dag, []gomel.PrivateKey) helpers.DagVerifier { return checkIfUndecidedVerifier },
		stopCondition,
	)

	return helpers.TestUsingTestRandomSource(pubKeys, privKeys, configurations, testingRoutine)
}

var _ = Describe("Byzantine Dag Test", func() {
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
		Context("by calling creating on a bigger dag", func() {
			It("should finish without errors", func() {
				err := testForkingChangingParents(createForkUsingNewUnit())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("by randomly choosing parents", func() {
			It("should finish without errors", func() {
				const parentsInForkingUnits = 2
				rand := rand.New(rand.NewSource(time.Now().UnixNano()))
				err := testForkingChangingParents(createForkWithRandomParents(parentsInForkingUnits, rand))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Cheat the deterministic part of the algorithm", func() {
		It("it should not be able to decide regarding a selected unit", func() {
			err := testLongTimeUndecidedStrategy()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
