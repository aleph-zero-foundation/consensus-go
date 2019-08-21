package helpers

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	// "gitlab.com/alephledger/consensus-go/pkg/random/beacon"
	"gitlab.com/alephledger/consensus-go/pkg/random/coin"
)

const (
	maxParents = 2
)

// UnitCreator is a type of a function that given a list of dags attempts to create a new unit or returns an error otherwise.
type UnitCreator func([]gomel.Dag, []gomel.PrivateKey, []gomel.RandomSource) (gomel.Preunit, error)

// Creator is a type of a function that given a dag and some 'creator' attempts to build a valid unit.
type Creator func(dag gomel.Dag, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error)

// AddingHandler is a type of a function that given a list of dags and a unit handles adding of that unit with accordance to
// used strategy.
type AddingHandler func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error

// DagVerifier is a type of a function that is responsible for verifying if a given list of dags is in valid state.
type DagVerifier func([]gomel.Dag, []uint16, []config.Configuration, []gomel.RandomSource) error

// TestingRoutine describes a strategy for performing a test on a given set of dags.
type TestingRoutine struct {
	creator       func(dags []gomel.Dag, privKeys []gomel.PrivateKey) UnitCreator
	adder         func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler
	verifier      func(dags []gomel.Dag, privKeys []gomel.PrivateKey) DagVerifier
	stopCondition func(dags []gomel.Dag) bool
}

// CreateUnitCreator create an instance of UnitCreator.
func (test *TestingRoutine) CreateUnitCreator(dags []gomel.Dag, privKeys []gomel.PrivateKey) UnitCreator {
	return test.creator(dags, privKeys)
}

// CreateAddingHandler creates an instance of AddingHandler.
func (test *TestingRoutine) CreateAddingHandler(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler {
	return test.adder(dags, privKeys, rss)
}

// CreateDagVerifier creates an instance of DagVerifier.
func (test *TestingRoutine) CreateDagVerifier(dags []gomel.Dag, privKeys []gomel.PrivateKey) DagVerifier {
	return test.verifier(dags, privKeys)
}

// StopCondition creates an instance of a function that decides when a testing routine should be stopped.
func (test *TestingRoutine) StopCondition() func(dags []gomel.Dag) bool {
	return test.stopCondition
}

// NewDefaultDataContent creates an instance of []byte equal to [1, 2, 3, 4]. It is not intended to be a valid payload for a
// unit.
func NewDefaultDataContent() []byte {
	return []byte{1, 2, 3, 4}
}

const nUnits = 1000

// NewDefaultTestingRoutine creates an instance of TestingRoutine.
func NewDefaultTestingRoutine(
	creator func(dags []gomel.Dag, privKeys []gomel.PrivateKey) UnitCreator,
	adder func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler,
	verifier func(dags []gomel.Dag, privKeys []gomel.PrivateKey) DagVerifier,
) *TestingRoutine {
	unitsCreated := 0
	stopCondition := func([]gomel.Dag) bool {
		return unitsCreated >= nUnits
	}
	wrappedCreator := func(dags []gomel.Dag, privKeys []gomel.PrivateKey) UnitCreator {
		origCreator := creator(dags, privKeys)
		return func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) (gomel.Preunit, error) {
			pu, err := origCreator(dags, privKeys, rss)
			if err == nil {
				unitsCreated++
			}
			return pu, err
		}
	}
	return &TestingRoutine{wrappedCreator, adder, verifier, stopCondition}
}

// NewTestingRoutineWithStopCondition creates an instance of TestingRoutine.
func NewTestingRoutineWithStopCondition(
	creator func(dags []gomel.Dag, privKeys []gomel.PrivateKey) UnitCreator,
	adder func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler,
	verifier func(dags []gomel.Dag, privKeys []gomel.PrivateKey) DagVerifier,
	stopCondition func([]gomel.Dag) bool,
) *TestingRoutine {
	return &TestingRoutine{creator, adder, verifier, stopCondition}
}

// NewDefaultAdder creates an instance of AddingHandler that ads a given unit to all dags under test.
func NewDefaultAdder() AddingHandler {
	return func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		_, err := AddToDags(preunit, rss, dags)
		return err
	}
}

// NewNoOpAdder return an instance of 'AddingHandler' type that performs no operation.
func NewNoOpAdder() AddingHandler {
	return func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		return nil
	}
}

// NewDefaultCreator creates an instance of Creator that when called attempts to create a unit using default data.
func NewDefaultCreator(maxParents uint16) Creator {
	return func(dag gomel.Dag, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error) {
		pu, err := creating.NewUnit(dag, int(creator), int(maxParents), NewDefaultDataContent(), rs, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
			return nil, err
		}
		pu.SetSignature(privKey.Sign(pu))
		return pu, nil
	}
}

// AddToDag is a helper method for synchronous addition of a unit to a given dag.
func AddToDag(dag gomel.Dag, pu gomel.Preunit, rs gomel.RandomSource) (gomel.Unit, error) {
	var result gomel.Unit
	var caughtError error
	var wg sync.WaitGroup
	wg.Add(1)
	dag.AddUnit(pu, rs, func(pu gomel.Preunit, u gomel.Unit, err error) {
		result = u
		caughtError = err
		wg.Done()
	})
	wg.Wait()
	return result, caughtError
}

// AddToDags is a helper function that adds a given unit to all provided dags.
func AddToDags(unit gomel.Preunit, rss []gomel.RandomSource, dags []gomel.Dag) (gomel.Unit, error) {
	var resultUnit gomel.Unit
	for ix, dag := range dags {
		result, err := AddToDag(dag, unit, rss[ix])
		if err != nil {
			return nil, err
		}
		if ix == unit.Creator() || resultUnit == nil {
			resultUnit = result
		}
	}
	return resultUnit, nil
}

// AddToDagsIngoringErrors adds a unit to all dags ignoring all errors while doing it. It returns, if possible, a Unit added the owning dag (assuming that order of 'dags' lists corresponds with their ids).
func AddToDagsIngoringErrors(unit gomel.Preunit, rss []gomel.RandomSource, dags []gomel.Dag) gomel.Unit {
	var resultUnit gomel.Unit
	for ix, dag := range dags {
		result, err := AddToDag(dag, unit, rss[ix])
		if resultUnit == nil {
			if result != nil {
				resultUnit = result
			} else if _, ok := err.(*gomel.DuplicateUnit); ok {
				duplicates := dag.Get([]*gomel.Hash{unit.Hash()})
				if len(duplicates) > 0 {
					resultUnit = duplicates[0]
				}
			}
		}
		if err != nil {
			if _, ok := err.(*gomel.DuplicateUnit); ok {
				continue
			}
			if _, ok := err.(*gomel.DataError); ok {
				fmt.Println("error while adding a unit (error was ignored):", err.Error())
				fmt.Printf("%+v\n", unit)
				for _, dag := range dags {
					parents := dag.Get(unit.Parents())
					if parents == nil {
						fmt.Println("missing parents")
						continue
					}
					failed := false
					for ix, parent := range parents {
						if parent == nil {
							fmt.Println("missing parent:", ix)
							failed = true
							break
						}
					}
					if failed {
						continue
					}
					fmt.Println("parents:")
					for _, parent := range parents {
						fmt.Printf("%+v\n", parent)
					}
				}
			}
		}
	}
	return resultUnit
}

// AddUnitsToDagsInRandomOrder adds a set of units in random order (per each dag) to all provided dags.
func AddUnitsToDagsInRandomOrder(units []gomel.Preunit, dags []gomel.Dag, rss []gomel.RandomSource) error {
	for ix, dag := range dags {
		rand.Shuffle(len(units), func(i, j int) {
			units[i], units[j] = units[j], units[i]
		})

		for _, pu := range units {
			if _, err := AddToDag(dag, pu, rss[ix]); err != nil {
				return err
			}
		}
	}
	return nil
}

// ComputeLevel computes value of the level attribute for a given preunit.
func ComputeLevel(dag gomel.Dag, parents []gomel.Unit) uint64 {
	if len(parents) == 0 {
		return 0
	}
	level := uint64(0)
	nProcesses := dag.NProc()
	for _, parent := range parents {
		if pl := parent.Level(); uint64(pl) > level {
			level = uint64(pl)
		}
	}
	nSeen := uint64(0)
	for pid := range parents[0].Floor() {
		pidFound := false
		for _, parent := range parents {
			for _, unit := range parent.Floor()[pid] {
				if uint64(unit.Level()) == level {
					nSeen++
					pidFound = true
					if dag.IsQuorum(int(nSeen)) {
						return level + 1
					}
					break
				}
			}
			if pidFound {
				break
			}
		}
		if !pidFound && !dag.IsQuorum(int(nSeen)+(nProcesses-(pid+1))) {
			break
		}
	}
	return level
}

// GenerateKeys is a helper function that creates a list of pairs of public-private keys.
func GenerateKeys(nProcesses int) ([]gomel.PublicKey, []gomel.PrivateKey) {
	pubKeys := make([]gomel.PublicKey, 0, nProcesses)
	privKeys := make([]gomel.PrivateKey, 0, nProcesses)
	for i := 0; i < nProcesses; i++ {
		pubKey, privKey, _ := signing.GenerateKeys()
		pubKeys = append(pubKeys, pubKey)
		privKeys = append(privKeys, privKey)
	}
	return pubKeys, privKeys
}

// NewDefaultConfigurations creates a slice of a given size containing default configurations.
func NewDefaultConfigurations(nProcesses int) []config.Configuration {
	defaultConfig := config.NewDefaultConfiguration()
	configs := make([]config.Configuration, nProcesses)
	for pid := range configs {
		configs[pid] = defaultConfig
	}
	return configs
}

// NewDefaultUnitCreator returns an implementation of the UnitCreator type that tries to build a unit using a randomly selected
// dag.
func NewDefaultUnitCreator(unitFactory Creator) UnitCreator {
	return func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) (gomel.Preunit, error) {
		attempts := 0
		for {
			attempts++
			if attempts%50 == 0 {
				fmt.Println("Attempt no", attempts, "of creating a new unit")
			}

			creator := rand.Intn(len(dags))
			dag := dags[creator]

			pu, err := unitFactory(dag, uint16(creator), privKeys[creator], rss[creator])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
				continue
			}
			if pu == nil {
				fmt.Fprintf(os.Stderr, "Creator %d was unable to build a unit\n", creator)
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))
			fmt.Fprintf(os.Stderr, "Unit created by dag no %d", creator)
			fmt.Fprintln(os.Stderr, "")
			return pu, nil
		}
	}
}

// NewEachInSequenceUnitCreator returns an instance of type UnitCreator that on every call attempts to create a new unit using
// a creator which is a direct successor of the previous one (i.e. 0, 1, 2...).
func NewEachInSequenceUnitCreator(unitFactory Creator) UnitCreator {
	nextCreator := 0
	return func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) (gomel.Preunit, error) {
		attempts := 0
		for {
			attempts++
			if attempts%50 == 0 {
				fmt.Println("Attempt no", attempts, "of creating a new unit")
			}

			creator := nextCreator
			dag := dags[creator]

			pu, err := unitFactory(dag, uint16(creator), privKeys[creator], rss[creator])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
				continue
			}
			if pu == nil {
				fmt.Fprintf(os.Stderr, "Creator %d was unable to build a unit\n", creator)
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))
			fmt.Fprintf(os.Stderr, "Unit created by dag no %d", creator)
			fmt.Fprintln(os.Stderr, "")

			nextCreator = (nextCreator + 1) % len(dags)
			return pu, nil
		}
	}
}

func getOrderedUnits(dag gomel.Dag, pid uint16, generalConfig config.Configuration, rs gomel.RandomSource) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {
		logger, _ := logging.NewLogger("stdout", generalConfig.LogLevel, 100000, false)
		ordering := linear.NewOrdering(dag, rs, int(generalConfig.VotingLevel), int(generalConfig.PiDeltaLevel), int(generalConfig.OrderStartLevel), generalConfig.CRPFixedPrefix, logger)
		level := 0
		orderedUnits := ordering.TimingRound(level)
		for orderedUnits != nil {
			for _, unit := range orderedUnits {
				units <- unit
			}
			level++
			orderedUnits = ordering.TimingRound(level)
		}
		dagLevel := dagLevel(dag)
		fmt.Printf("Dag's no %d max level: %d", pid, dagLevel)
		fmt.Println()

		close(units)
	}()
	return units
}

func getAllTimingUnits(dag gomel.Dag, pid uint16, generalConfig config.Configuration, rs gomel.RandomSource) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {

		logger, _ := logging.NewLogger("stdout", generalConfig.LogLevel, 100000, false)
		ordering := linear.NewOrdering(dag, rs, int(generalConfig.VotingLevel), int(generalConfig.PiDeltaLevel), generalConfig.OrderStartLevel, generalConfig.CRPFixedPrefix, logger)
		level := 0
		timingUnit := ordering.DecideTiming()
		for timingUnit != nil {
			units <- timingUnit
			level++
			timingUnit = ordering.DecideTiming()
		}
		fmt.Printf("maximal decided level of dag no %d: %d", pid, level)
		fmt.Println()
		close(units)
	}()
	return units
}

func getMaximalUnitsSorted(dag gomel.Dag, pid uint16, generalConfig config.Configuration, rs gomel.RandomSource) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {
		dag.MaximalUnitsPerProcess().Iterate(func(forks []gomel.Unit) bool {
			// order of 'forks' list might be different for different dags depending on order in which units were add to it
			// sort all forks using their Hash values
			sorted := make([]gomel.Unit, len(forks))
			copy(sorted, forks)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Hash().LessThan(sorted[j].Hash())
			})
			for _, unit := range sorted {
				units <- unit
			}
			return true
		})
		close(units)
	}()
	return units
}

func dagLevel(dag gomel.Dag) uint64 {
	result := uint64(0)
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, unit := range units {
			if level := uint64(unit.Level()); level > result {
				result = level
			}
		}
		return true
	})
	return result
}

// ComposeVerifiers composes provided verifiers into a single verifier. Created verifier fails immediately after it discovers a failure of one of
// its verifiers.
func ComposeVerifiers(verifiers ...DagVerifier) DagVerifier {
	return func(dags []gomel.Dag, pids []uint16, generalConfigs []config.Configuration, rss []gomel.RandomSource) error {
		for _, verifier := range verifiers {
			if err := verifier(dags, pids, generalConfigs, rss); err != nil {
				return err
			}
		}
		return nil
	}
}

// ComposeAdders composes provided 'adders' into a single 'adder'. Created 'adder' fails immediately after it discovers a
// failure of one of its 'adders'.
func ComposeAdders(adders ...AddingHandler) AddingHandler {

	return func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		for _, adder := range adders {
			if err := adder(dags, rss, preunit); err != nil {
				return err
			}
		}
		return nil
	}
}

func verifyUnitsUsingOrdering(ordering func(gomel.Dag, uint16, config.Configuration, gomel.RandomSource) chan gomel.Unit, checker func(u1, u2 gomel.Unit) error) DagVerifier {
	return func(dags []gomel.Dag, pids []uint16, generalConfigs []config.Configuration, rss []gomel.RandomSource) error {
		if len(dags) < 2 {
			return nil
		}
		var units1 []gomel.Unit
		for unit := range ordering(dags[0], pids[0], generalConfigs[0], rss[0]) {
			units1 = append(units1, unit)
		}
		for ix, dag := range dags {
			units2 := ordering(dag, pids[ix], generalConfigs[ix], rss[ix])

			for _, unit1 := range units1 {
				unit2, open := <-units2

				if !open {
					return gomel.NewDataError(fmt.Sprintf("dag id=%d returned more units than dag id=%d", 0, ix))
				}

				if err := checker(unit1, unit2); err != nil {
					return err
				}
			}

			if _, open := <-units2; open {
				return gomel.NewDataError(fmt.Sprintf("dag id=%d returned more units than dag id=%d", ix, 0))
			}

		}
		return nil
	}
}

// VerifyTimingUnits returns a dag verifier that checks if all dags returns same set of timing units.
func VerifyTimingUnits() DagVerifier {
	prevLevel := -1
	return verifyUnitsUsingOrdering(

		func(dag gomel.Dag, pid uint16, generalConfig config.Configuration, rs gomel.RandomSource) chan gomel.Unit {
			prevLevel = -1
			return getAllTimingUnits(dag, pid, generalConfig, rs)
		},

		func(u1, u2 gomel.Unit) error {
			level := u1.Level()
			if level != prevLevel+1 {
				return gomel.NewDataError(
					fmt.Sprintf("Missing timing unit for level %d - obtained %d. Unit: %+v", prevLevel+1, level, u1),
				)
			}
			prevLevel = level

			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Dags selected different timing units")
			}
			return nil
		},
	)
}

// VerifyOrdering returns a DagVerifier that compares if all dags orders their underlying units in the same way.
func VerifyOrdering() DagVerifier {
	return verifyUnitsUsingOrdering(

		getOrderedUnits,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Dags differ in ordering")
			}
			return nil
		},
	)
}

// VerifyAllDagsContainSameMaximalUnits returns a DagVerifier that checks if all dags provide same set of maximal units.
func VerifyAllDagsContainSameMaximalUnits() DagVerifier {
	return verifyUnitsUsingOrdering(
		getMaximalUnitsSorted,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				fmt.Printf("u1 %+v\n", u1)
				fmt.Printf("u2 %+v\n", u2)
				return gomel.NewDataError("dags contains different maximal units")
			}
			return nil
		},
	)
}

// NewDefaultVerifier returns a DagVerifier composed from VerifyAllDagsContainSameMaximalUnits, VerifyTimingUnits and
// VerifyOrdering verifiers.
func NewDefaultVerifier() func([]gomel.Dag, []gomel.PrivateKey) DagVerifier {
	return func(dags []gomel.Dag, privKeys []gomel.PrivateKey) DagVerifier {
		return ComposeVerifiers(VerifyAllDagsContainSameMaximalUnits(), VerifyTimingUnits(), VerifyOrdering())
	}
}

// NewNoOpVerifier returns a DagVerifier that does not check provided dags and immediately answers that they are correct.
func NewNoOpVerifier() DagVerifier {
	return func([]gomel.Dag, []uint16, []config.Configuration, []gomel.RandomSource) error {
		fmt.Println("No verification step")
		return nil
	}
}

// Test is a helper function that performs a single test using provided TestingRoutineFactory.
func Test(
	pubKeys []gomel.PublicKey,
	privKeys []gomel.PrivateKey,
	configurations []config.Configuration,
	testingRoutine *TestingRoutine,
) error {

	nProcesses := len(pubKeys)
	dags := make([]gomel.Dag, 0, nProcesses)
	pids := make([]uint16, 0, nProcesses)
	rss := make([]gomel.RandomSource, 0, nProcesses)

	for pid := uint16(0); len(dags) < nProcesses; pid++ {
		dag := growing.NewDag(&gomel.DagConfig{Keys: pubKeys})
		defer dag.Stop()
		dags = append(dags, dag)

		pids = append(pids, pid)
		rs := coin.NewFixedCoin(nProcesses, int(pid), 0)
		rs.Init(dag)
		rss = append(rss, rs)
	}

	unitCreator, addingHandler, verifier, stopCondition :=
		testingRoutine.CreateUnitCreator(dags, privKeys),
		testingRoutine.CreateAddingHandler(dags, privKeys, rss),
		testingRoutine.CreateDagVerifier(dags, privKeys),
		testingRoutine.StopCondition()

	fmt.Println("Starting a testing routine")
	for !stopCondition(dags) {

		var newUnit gomel.Preunit
		var err error
		if newUnit, err = unitCreator(dags, privKeys, rss); err != nil {
			fmt.Fprintln(os.Stderr, "Unable to create a new unit")
			return err
		}

		// send the unit to all dags
		if err := addingHandler(dags, rss, newUnit); err != nil {
			fmt.Fprintln(os.Stderr, "Error while adding a unit to some dag:", err)
			return err
		}
	}
	fmt.Println("Testing routine finished")

	fmt.Println("Verification step")
	err := verifier(dags, pids, configurations, rss)
	if err != nil {
		fmt.Println("Dags verfication failed", err.Error())
		return err
	}
	fmt.Println("Verification step finished")
	return nil
}
