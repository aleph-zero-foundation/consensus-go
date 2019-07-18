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
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/random"
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
		preunit.Parents(),
		data,
		preunit.RandomSourceData(),
	)
}

func newForkerUsingDifferentDataStrategy() forker {
	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		return newForkWithDifferentData(preunit), nil
	}
}

func createForkUsingNewUnit(parentsCount int) forker {
	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {

		pu, err := creating.NewUnit(dag, int(preunit.Creator()), parentsCount, helpers.NewDefaultDataContent(), rs, false)
		if err != nil {
			return nil, fmt.Errorf("unable to create a forking unit: %s", err.Error())
		}

		parents := pu.Parents()
		parents[0] = preunit.Parents()[0]
		freshData := generateFreshData(preunit.Data())
		parentUnits := dag.Get(parents)
		level := computeLevel(dag, parentUnits)
		rsData := rs.DataToInclude(pu.Creator(), parentUnits, int(level))
		return creating.NewPreunit(pu.Creator(), parents, freshData, rsData), nil
	}
}

func checkSelfForkingEvidence(parents []gomel.Unit, creator uint16) bool {
	var max gomel.Unit
	for ix, parent := range parents {
		if floor := parent.Floor()[creator]; len(floor) > 0 {
			if len(floor) > 1 {
				return false
			}
			max = floor[0]
			parents = parents[ix:]
			break
		}
	}
	if max == nil {
		return true
	}
	for _, parent := range parents {
		floor := parent.Floor()[creator]
		if len(floor) == 0 {
			continue
		}
		if len(floor) > 1 {
			return false
		}
		if max.Below(floor[0]) {
			max = floor[0]
		} else if !floor[0].Below(max) {
			return false
		}
	}
	return true
}

func checkCompliance(dag gomel.Dag, creator uint16, parents []gomel.Unit) error {
	if !checkSelfForkingEvidence(parents, creator) {
		return gomel.NewComplianceError("parents contain evidence of self forking")
	}
	if growing.CheckForkerMuting(parents) != nil {
		return gomel.NewComplianceError("parents do not satisfy the forker-muting rule")
	}
	if growing.CheckExpandPrimes(dag, parents) != nil {
		return gomel.NewComplianceError("parents violate the expand-primes rule")
	}
	return nil
}

func createForkWithRandomParents(parentsCount int, rand *rand.Rand) forker {

	return func(preunit gomel.Preunit, dag gomel.Dag, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {

		parents := make([]*gomel.Hash, 0, parentsCount)
		parentUnits := make([]gomel.Unit, 0, parentsCount)
		selfPredecessor := dag.Get([]*gomel.Hash{preunit.Parents()[0]})[0]
		parents = append(parents, selfPredecessor.Hash())
		parentUnits = append(parentUnits, selfPredecessor)

		for _, pid := range rand.Perm(dag.NProc()) {
			if len(parents) >= parentsCount {
				break
			}
			if pid == preunit.Creator() {
				continue
			}

			var availableParents []gomel.Unit
			availableParents = append(availableParents, dag.MaximalUnitsPerProcess().Get(pid)...)

			for len(availableParents) > 0 {
				randIx := rand.Intn(len(availableParents))
				selectedParent := availableParents[randIx]
				availableParents[len(availableParents)-1], availableParents[randIx] =
					availableParents[randIx], availableParents[len(availableParents)-1]
				parentUnits = append(parentUnits, selectedParent)
				if err := checkCompliance(dag, uint16(preunit.Creator()), parentUnits); err != nil {
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
				break
			}
		}
		if len(parents) < 2 {
			return nil, errors.New("unable to collect enough parents")
		}
		freshData := generateFreshData(preunit.Data())
		level := computeLevel(dag, parentUnits)
		rsData := rs.DataToInclude(preunit.Creator(), parentUnits, int(level))
		return creating.NewPreunit(preunit.Creator(), parents, freshData, rsData), nil
	}
}

func computeLevel(dag gomel.Dag, parents []gomel.Unit) uint64 {
	level := uint64(0)
	for _, parent := range parents {
		if pl := parent.Level(); uint64(pl) > level {
			level = uint64(pl)
		}
	}
	onLevel := map[uint16]bool{}
	for _, parent := range parents {
		for pid, floor := range parent.Floor() {
			for _, unit := range floor {
				if uint64(unit.Level()) == level {
					onLevel[uint16(pid)] = true
					if dag.IsQuorum(len(onLevel)) {
						return level + 1
					}
				}
			}
		}
	}
	return level
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

func newDefaultUnitCreator(maxParents uint16) helpers.Creator {
	return func(dag gomel.Dag, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		pu, err := creating.NewUnit(dag, int(creator), int(maxParents), helpers.NewDefaultDataContent(), rs, false)
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

	defaultUnitCreator := newDefaultUnitCreator(maxParents)
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
		if forkingRoot != nil && uint64(unit.Level()) >= buildLevel && createdForks == nil && uint16(unit.Creator()) != forker {
			createdForks = forkingStrategy(forkingRoot, dags[forker], privKey, rss[forker], numberOfForks)
		}

		// add forking units to all dags
		if len(createdForks) > 0 && uint64(unit.Level()) >= showOffLevel {
			// show all created forks to all participants
			if err := helpers.AddUnitsToDagsInRandomOrder(createdForks, dags, rss); err != nil {
				return err
			}
			fmt.Println("Byzantine node added a fork:", createdForks[0].Creator())
			alreadyAdded = true
		}
		return nil
	}
	return unitCreator, addingHandler, nil
}

func computeMaxPossibleNumberOfByzantineProcesses(nProc int) int {
	return (nProc - 1) / 3
}

func getRandomListOfByzantineDags(n int) []int {
	byzProcesses := computeMaxPossibleNumberOfByzantineProcesses(n)
	return rand.Perm(byzProcesses)[:byzProcesses]
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

func newSimpleForkingAdder(forkingLevel int, privKeys []gomel.PrivateKey, byzantineDags []int, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

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
				err := helpers.AddUnitsToDagsInRandomOrder(units, dags, rss)
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

func newPrimeFloodingAdder(floodingLevel int, numberOfPrimes int, privKeys []gomel.PrivateKey, byzantineDags []int, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

	return func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) helpers.AddingHandler {
		alreadyFlooded := make(map[int]bool, len(byzantineDags))
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

func newRandomForkingAdder(byzantineDags []int, forkProbability int, privKeys []gomel.PrivateKey, forkingStrategy forkingStrategy) func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {

	return func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler {
		forkers := make(map[int]bool, len(byzantineDags))
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
	unitCreator := helpers.NewDefaultUnitCreator(newDefaultUnitCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newPrimeFloodingAdder(floodingLevel, forkingPrimes, privKeys, byzantineDags, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, testingRoutine)
}

func testSimpleForkingScenario(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)
	unitCreator := helpers.NewDefaultUnitCreator(newDefaultUnitCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newSimpleForkingAdder(10, privKeys, byzantineDags, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, testingRoutine)
}

func testRandomForking(forkingStrategy forkingStrategy) error {
	const (
		nProcesses = 21
		nUnits     = 1000
		maxParents = 2
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(newDefaultUnitCreator(maxParents))
	byzantineDags := getRandomListOfByzantineDags(nProcesses)
	unitAdder := newRandomForkingAdder(byzantineDags, 50, privKeys, forkingStrategy)
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		unitAdder,
		verifier,
	)

	return helpers.Test(pubKeys, privKeys, testingRoutine)
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

			uint16(byzDag),
			privKeys[byzDag],
			createForksUsingForker(forker),
			numberOfForks,
			maxParents,
		)

		if err != nil {
			return err
		}

		byzDags[uint16(byzDag)] = byzPair{unitCreator, addingHandler}
	}

	defaultUnitCreator := newDefaultUnitCreator(maxParents)
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

	return helpers.Test(pubKeys, privKeys, testingRoutine)
}

// NOTE this was copied from voting.go
func simpleCoin(u gomel.Unit, level int) int {
	index := level % (8 * len(u.Hash()))
	byteIndex, bitIndex := index/8, index%8
	if u.Hash()[byteIndex]&(1<<uint(bitIndex)) > 0 {
		return 1
	}
	return 0
}

func newDefaultCommonVote(uc gomel.Unit, initialVotingRound uint64, lastDeterministicRound uint64) <-chan bool {
	commonVotes := make(chan bool)
	go func() {
		commonVotes <- true
		commonVotes <- false

		// use the simplecoin to predict future common votes
		lastLevel := uc.Level() + int(initialVotingRound) + int(lastDeterministicRound)
		for level := uc.Level() + int(initialVotingRound) + 3; level < lastLevel; level++ {
			if simpleCoin(uc, level) == 0 {
				commonVotes <- true
			} else {
				commonVotes <- false
			}
		}
		close(commonVotes)
	}()
	return commonVotes
}

func syncDags(p1, p2 gomel.Dag, rs1, rs2 gomel.RandomSource) (bool, error) {
	p1Max := p1.MaximalUnitsPerProcess()
	p2Max := p2.MaximalUnitsPerProcess()
	missingForP2 := map[gomel.Unit]bool{}
	missingForP1 := map[gomel.Unit]bool{}
	for pid := 0; pid < p1.NProc(); pid++ {
		p1Units := append([]gomel.Unit(nil), p1Max.Get(pid)...)
		p2Units := append([]gomel.Unit(nil), p2Max.Get(pid)...)

		different := func(units []gomel.Unit, dag gomel.Dag) map[gomel.Unit]bool {
			missing := map[gomel.Unit]bool{}
			for _, unit := range units {
				// descend to a common parent
				current := unit
				for current != nil {
					other := dag.Get([]*gomel.Hash{current.Hash()})
					if len(other) < 1 || other[0] == nil {
						if missing[current] {
							break
						} else {
							missing[current] = true
						}
					} else {
						break
					}

					predecessor, err := gomel.Predecessor(current)
					if err != nil || predecessor == nil {
						current = nil
					} else {
						current = predecessor
					}
				}
			}
			return missing
		}
		for unit := range different(p1Units, p2) {
			if missingForP2[unit] {
				break
			}
			missingForP2[unit] = true
		}
		for unit := range different(p2Units, p1) {
			if missingForP1[unit] {
				break
			}
			missingForP1[unit] = true
		}
	}

	// sort units topologically
	missingForP1Slice := topoSort(missingForP1)
	missingForP2Slice := topoSort(missingForP2)

	adder := func(units []gomel.Unit, dag gomel.Dag, rs gomel.RandomSource) error {
		for _, unit := range units {
			preunit := unitToPreunit(unit)
			_, err := helpers.AddToDag(dag, preunit, rs)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if err := adder(missingForP1Slice, p1, rs1); err != nil {
		return true, err
	}
	if err := adder(missingForP2Slice, p2, rs2); err != nil {
		return true, err
	}
	return len(missingForP1) > 0 || len(missingForP2) > 0, nil
}

// TopoSort sort units topologically.
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

func countUnitsOnLevelOrHigher(dag gomel.Dag, level uint64) map[uint16]bool {
	seen := make(map[uint16]bool, dag.NProc())
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, unit := range units {
			if uint64(unit.Level()) >= level {
				seen[uint16(unit.Creator())] = true
				if dag.IsQuorum(len(seen)) {
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

// Maintain two independent 'towers', first consisting of process voting for `1` and the other voting for `0`. Depending on the
// value of the next common vote, their sizes are 'f' and 'N-f' respectively. For example, when the value of the next common
// vote equals 1, then 1's tower consist of 'f' processes and the 0's tower of 'N-f'. This way the decision procedure is unable
// to commit its value - common vote is different from the value that votes 1 (other side votes 'undecided' and so chooses the
// common vote as its value). For simplicity, lets assume we are currently building units for one of the towers consisting of
// 'N-f' processes. We keep building new levels on the same side until the value of the next common vote forces us to switch
// sides, i.e. during the last round we created only 'f' prime units and so we are unable to create a new level without looking
// at units from the other 'tower'. While building consecutive levels we enqueue all of the created units for the opposite side,
// so after current side finishes or switches, the other one can reveal them one by one till it is able to jump on some level.
// We also enqueue all read votes, since the other side will not be able to read them from the provided channel after we
// processed them.

// The initialization process, that is a few levels before the initial voting, is little bit tricky. We need to ensure that a
// unit U_c for which we are deciding becomes popular before the initial voting starts, but also allow at most 2f processes to
// record that it is popular on the voting level. This way it will not be decided 0 (as well as 1) at the round no
// initialVoting+1. We achieve this goal by two means: extending common votes by some initial values and reverting the list
// of processes that vote 1 on level initialVoting-1. Former allows us to treat the initialization similar way as any other
// round. For details, see the `fixCommonVotes` function. Later, makes the unit U_c not being decided 1 by the 'fast' algorithm.
// To this point, the extended common vote (0 for round `initialVoting`) forces us to make the unit U_c popular on level
// `initialVoting-1` (subset of processes voting 1 being of size 2f+1). If we would not reverse processes on the 1's tower list,
// then there would be a chance that processes on the zeros side would construct a proof of popularity of U_c, i.e. f+1 nodes
// that are shared between 1's and 0's from round `initialVoting-1` would introduce, having them as parents, f new nodes that
// are above U_c from ones side which would give us a quorum of processes that see U_c. After we reverse the 1's side, the 0's
// side uses f shared nodes from round initialVoting-1 that are not able to introduce any new processes (till that round they
// were building their levels alone using nodes from 0's side) and one additional processes that can only introduce f processes
// above U_c that we already know, giving us f+1 processes above U_c in total.

// *tower:                 1 (votes 1) tower        0 tower
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
	initialVotingRound uint64,
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
		r uint64
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
	level := uint64(decisionUnit.Level()) - 1
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

	checkMissing := func(level uint64, seen map[uint16]bool, source []pair) map[uint16]bool {

		if dags[0].IsQuorum(len(seen)) {
			return seen
		}

		for _, newUnit := range source {

			if newUnit.r > level {
				break
			}

			if newUnit.r == level {
				seen[uint16(newUnit.l.Creator())] = true
				if dags[0].IsQuorum(len(seen)) {
					return seen
				}
			}
		}

		return seen
	}

	addMissing := func(level uint64, seen map[uint16]bool, source *[]pair, sink []gomel.Dag, sinkRss []gomel.RandomSource) map[uint16]bool {

		for !dags[0].IsQuorum(len(seen)) && len(*source) > 0 {

			newUnit := (*source)[0]
			if newUnit.r > level {
				break
			}
			addedUnit := helpers.AddToDagsIngoringErrors(newUnit.l, sinkRss, sink)
			*source = (*source)[1:]
			if addedUnit != nil && uint64(addedUnit.Level()) == level {
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
			if uint64(unit.Level()) >= level {
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
		if !dags[0].IsQuorum(len(seen)) {
			seen = checkMissing(level, seen, *awaitingUnits)
		}
		if !dags[0].IsQuorum(len(seen)) {
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
		} else {
			rightSide = dags[ones:]
			rightKeys = privateKeys[ones:]
			rightIds = ids[ones:]
		}

		// synchronize all dags for the current round
		err := syncAllDags(*currentSet, *currentRss)
		if err != nil {
			return err
		}

		seen = countUnitsOnLevelOrHigher((*currentSet)[0], level)
		// show minimal number of units that allows us to build new level
		seen = addMissing(level, seen, myUnits, *currentSet, *currentRss)
		if !dags[0].IsQuorum(len(seen)) {
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
		if level+1 == uint64(decisionUnit.Level())+initialVotingRound {
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
	var maxNumberOfByzantineProcesses = uint16(computeMaxPossibleNumberOfByzantineProcesses(int(nProc)))
	var nonByzantineProcesses = nProc - maxNumberOfByzantineProcesses
	if vote {
		return maxNumberOfByzantineProcesses, nonByzantineProcesses
	}
	return nonByzantineProcesses, maxNumberOfByzantineProcesses
}

func unitToPreunit(unit gomel.Unit) gomel.Preunit {
	hashParents := make([]*gomel.Hash, 0, len(unit.Parents()))
	for _, parent := range unit.Parents() {
		hashParents = append(hashParents, parent.Hash())
	}
	return creating.NewPreunit(unit.Creator(), hashParents, unit.Data(), unit.RandomSourceData())
}

// this function assumes that it is possible to create a new level, i.e. there are enough candidates on lower level
func buildOneLevelUp(
	dags []gomel.Dag,
	privKeys []gomel.PrivateKey,
	ids []uint16,
	rss []gomel.RandomSource,
	level uint64,
) ([]gomel.Preunit, error) {
	// IMPORTANT instead of adding units immediately to all dags, save them so they will be processed on the next round

	createdUnits := make([]gomel.Preunit, 0, len(dags))
	ones, _ := subsetSizesOfLowerLevelBasedOnCommonVote(true, uint16(dags[0].NProc()))
	for ix, dag := range dags {
		createdOnLevel := false
		for !createdOnLevel {
			// check if process/dag is already on that level
			for _, unit := range dag.MaximalUnitsPerProcess().Get(int(ids[ix])) {
				if unit.Level() == int(level) {
					preunit := unitToPreunit(unit)
					createdUnits = append(createdUnits, preunit)
					createdOnLevel = true
					break
				}
			}
			if createdOnLevel {
				break
			}
			preunit, err := creating.NewUnit(dag, int(ids[ix]), int(ones), helpers.NewDefaultDataContent(), rss[ix], true)
			if err != nil {
				return nil, fmt.Errorf("error while creating a unit for dag no %d: %s", ids[ix], err.Error())
			}
			// add only to its creator's dag
			addedUnit, err := helpers.AddToDag(dag, preunit, rss[ix])
			if err != nil {
				return nil, fmt.Errorf("error while adding to dag no %d: %s", ids[ix], err.Error())
			}

			createdUnits = append(createdUnits, preunit)
			if uint64(addedUnit.Level()) == level {
				createdOnLevel = true
			}
		}
	}
	return createdUnits, nil
}

func fixCommonVotes(commonVotes <-chan bool, initialVotingRound uint64) <-chan bool {
	fixedVotes := make(chan bool)
	go func() {
		for ix := uint64(0); ix < initialVotingRound-1; ix++ {
			fixedVotes <- true
		}
		// this vote forces the "decision" unit to become popular exactly on the voting level
		// otherwise it would be decided 0
		fixedVotes <- false
		for vote := range commonVotes {
			fixedVotes <- vote
		}
		close(fixedVotes)
	}()
	return fixedVotes
}

func longTimeUndecidedStrategy(startLevel uint64, initialVotingRound uint64, numberOfDeterministicRounds uint64) (func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) helpers.AddingHandler, func([]gomel.Dag) bool) {

	alreadyTriggered := false
	resultAdder := func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) helpers.AddingHandler {
		seen := make(map[uint16]bool, len(dags))
		triggerCondition := func(unit gomel.Unit) bool {
			if alreadyTriggered {
				return false
			}
			if uint64(unit.Level()) == (startLevel - 1) {
				seen[uint16(unit.Creator())] = true
			}
			if dags[0].IsQuorum(len(seen)) && unit.Creator() != rss[0].GetCRP(int(startLevel))[0] {
				return true
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
			triggeringDag := uint16(rss[0].GetCRP(int(startLevel))[0])
			fmt.Println("triggering dag no", triggeringDag)
			triggeringPreunit, err := creating.NewUnit(
				dags[triggeringDag],
				int(triggeringDag),
				len(dags),
				helpers.NewDefaultDataContent(),
				rss[triggeringDag], false,
			)
			if err != nil {
				return err
			}
			triggeringUnit, err := helpers.AddToDag(dags[triggeringDag], triggeringPreunit, rss[triggeringDag])
			if err != nil {
				return err
			}
			commonVotes := newDefaultCommonVote(triggeringUnit, initialVotingRound, numberOfDeterministicRounds)

			dagsCopy[triggeringDag], dagsCopy[0] = dagsCopy[0], dagsCopy[triggeringDag]
			privKeysCopy[triggeringDag], privKeysCopy[0] = privKeysCopy[0], privKeysCopy[triggeringDag]
			ids[triggeringDag], ids[0] = ids[0], ids[triggeringDag]
			rssCopy[triggeringDag], rssCopy[0] = rssCopy[0], rssCopy[triggeringDag]

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
		maxParents                  = 2
		startLevel                  = uint64(10)
		initialVotingRound          = uint64(3)
		numberOfDeterministicRounds = uint64(60)
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)

	unitCreator := helpers.NewDefaultUnitCreator(newDefaultUnitCreator(maxParents))

	unitAdder, stopCondition := longTimeUndecidedStrategy(startLevel, initialVotingRound, numberOfDeterministicRounds)

	checkIfUndecidedVerifier := func(dags []gomel.Dag, pids []uint16, configs []config.Configuration) error {
		fmt.Println("starting the undecided checker")

		config := config.NewDefaultConfiguration()
		config.VotingLevel = uint(initialVotingRound)
		config.PiDeltaLevel = uint(numberOfDeterministicRounds + 1)

		errorsCount := 0
		for pid, dag := range dags {
			rs := random.NewTcSource(dag, pid)
			ordering := linear.NewOrdering(dag, rs, int(config.VotingLevel), int(config.PiDeltaLevel))
			if unit := ordering.DecideTimingOnLevel(int(startLevel)); unit != nil {
				fmt.Println("some dag already decided - error")
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

	return helpers.Test(pubKeys, privKeys, testingRoutine)
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
				const parentsInForkingUnits = 2
				err := testForkingChangingParents(createForkUsingNewUnit(parentsInForkingUnits))
				Expect(err).NotTo(HaveOccurred())
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
