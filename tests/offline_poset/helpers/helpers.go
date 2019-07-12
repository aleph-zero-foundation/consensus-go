package helpers

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/random"
)

const (
	maxParents = 2
)

// UnitCreator is a type of a function that given a list of posets attempts to create a new unit or returns an error otherwise.
type UnitCreator func([]gomel.Poset, []gomel.PrivateKey, []gomel.RandomSource) (gomel.Preunit, error)

// Creator is a type of a function that given a poset and some 'creator' attempts to build a valid unit.
type Creator func(poset gomel.Poset, creator uint16, privKey gomel.PrivateKey, rs gomel.RandomSource) (gomel.Preunit, error)

// AddingHandler is a type of a function that given a list of posets and a unit handles adding of that unit with accordance to
// used strategy.
type AddingHandler func(posets []gomel.Poset, rss []gomel.RandomSource, preunit gomel.Preunit) error

// PosetVerifier is a type of a function that is responsible for verifying if a given list of posests is in valid state.
type PosetVerifier func([]gomel.Poset, []uint16, []config.Configuration) error

// TestingRoutine describes a strategy for performing a test on a given set of posets.
type TestingRoutine interface {
	CreateUnitCreator(posets []gomel.Poset, privKeys []gomel.PrivateKey) UnitCreator
	CreateAddingHandler(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler
	CreatePosetVerifier(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier
	StopCondition() func(posets []gomel.Poset) bool
}

type testingRoutine struct {
	creator       func(posets []gomel.Poset, privKeys []gomel.PrivateKey) UnitCreator
	adder         func(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler
	verifier      func(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier
	stopCondition func(posets []gomel.Poset) bool
}

func (test *testingRoutine) CreateUnitCreator(posets []gomel.Poset, privKeys []gomel.PrivateKey) UnitCreator {
	return test.creator(posets, privKeys)
}

func (test *testingRoutine) CreateAddingHandler(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler {
	return test.adder(posets, privKeys, rss)
}

func (test *testingRoutine) CreatePosetVerifier(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier {
	return test.verifier(posets, privKeys)
}

func (test *testingRoutine) StopCondition() func(posets []gomel.Poset) bool {
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
	creator func(posets []gomel.Poset, privKeys []gomel.PrivateKey) UnitCreator,
	adder func(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler,
	verifier func(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier,
) TestingRoutine {
	unitsCreated := 0
	stopCondition := func([]gomel.Poset) bool {
		return unitsCreated >= nUnits
	}
	return &testingRoutine{creator, adder, verifier, stopCondition}
}

// NewTestingRoutineWithStopCondition creates an instance of TestingRoutine.
func NewTestingRoutineWithStopCondition(
	creator func(posets []gomel.Poset, privKeys []gomel.PrivateKey) UnitCreator,
	adder func(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) AddingHandler,
	verifier func(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier,
	stopCondition func([]gomel.Poset) bool,
) TestingRoutine {
	return &testingRoutine{creator, adder, verifier, stopCondition}
}

// NewDefaultAdder creates an instance of AddingHandler that ads a given unit to all posets under test.
func NewDefaultAdder() AddingHandler {
	return func(posets []gomel.Poset, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		_, err := AddToPosets(preunit, posets)
		return err
	}
}

// NewNoOpAdder return an instance of 'AddingHandler' type that performs no operation.
func NewNoOpAdder() AddingHandler {
	return func(posets []gomel.Poset, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		return nil
	}
}

// AddToPoset is a helper method for synchronous addition of a unit to a given poset.
func AddToPoset(poset gomel.Poset, pu gomel.Preunit) (gomel.Unit, error) {
	var result gomel.Unit
	var caughtError error
	var wg sync.WaitGroup
	wg.Add(1)
	poset.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, err error) {
		result = u
		caughtError = err
		wg.Done()
	})
	wg.Wait()
	return result, caughtError
}

// AddToPosets is a helper function that adds a given unit to all provided posets.
func AddToPosets(unit gomel.Preunit, posets []gomel.Poset) (gomel.Unit, error) {
	var resultUnit gomel.Unit
	for ix, poset := range posets {
		result, err := AddToPoset(poset, unit)
		if err != nil {
			return nil, err
		}
		if ix == unit.Creator() || resultUnit == nil {
			resultUnit = result
		}
	}
	return resultUnit, nil
}

// AddToPosetsIngoringErrors adds a unit to all posets ignoring all errors while doing it. It returns, if possible, a Unit added the owning poset (assuming that order of 'posets' lists corresponds with their ids).
func AddToPosetsIngoringErrors(unit gomel.Preunit, posets []gomel.Poset) gomel.Unit {
	var resultUnit gomel.Unit
	for _, poset := range posets {
		result, err := AddToPoset(poset, unit)
		if resultUnit == nil {
			if result != nil {
				resultUnit = result
			} else if _, ok := err.(*gomel.DuplicateUnit); ok {
				duplicates := poset.Get([]*gomel.Hash{unit.Hash()})
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
				for _, poset := range posets {
					parents := poset.Get(unit.Parents())
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

// AddUnitsToPosetsInRandomOrder adds a set of units in random order (per each poset) to all provided posets.
func AddUnitsToPosetsInRandomOrder(units []gomel.Preunit, posets []gomel.Poset) error {
	for _, poset := range posets {
		rand.Shuffle(len(units), func(i, j int) {
			units[i], units[j] = units[j], units[i]
		})

		for _, pu := range units {
			if _, err := AddToPoset(poset, pu); err != nil {
				return err
			}
		}
	}
	return nil
}

// GenerateKeys is a helper function that creates a list of pairs of public-private keys.
func GenerateKeys(nProcesses int) (pubKeys []gomel.PublicKey, privKeys []gomel.PrivateKey) {
	pubKeys = make([]gomel.PublicKey, 0, nProcesses)
	privKeys = make([]gomel.PrivateKey, 0, nProcesses)
	for i := 0; i < nProcesses; i++ {
		pubKey, privKey, _ := signing.GenerateKeys()
		pubKeys = append(pubKeys, pubKey)
		privKeys = append(privKeys, privKey)
	}
	return pubKeys, privKeys
}

// NewDefaultUnitCreator returns an implementation of the UnitCreator type that tries to build a unit using a randomly selected
// poset.
func NewDefaultUnitCreator(unitFactory Creator) UnitCreator {
	return func(posets []gomel.Poset, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) (gomel.Preunit, error) {
		attempts := 0
		for {
			attempts++
			if attempts%50 == 0 {
				fmt.Println("Attempt no", attempts, "of creating a new unit")
			}

			creator := rand.Intn(len(posets))
			poset := posets[creator]

			// pu, err := creating.NewUnit(poset, creator, maxParents, NewDefaultDataContent())
			pu, err := unitFactory(poset, uint16(creator), privKeys[creator], rss[creator])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
				continue
			}
			if pu == nil {
				fmt.Fprintf(os.Stderr, "Creator %d was unable to build a unit\n", creator)
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))
			return pu, nil
		}
	}
}

func getOrderedUnits(poset gomel.Poset, pid uint16, generalConfig config.Configuration) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {
		// TODO types
		rs := random.NewTcSource(poset, int(pid))
		ordering := linear.NewOrdering(poset, rs, int(generalConfig.VotingLevel), int(generalConfig.PiDeltaLevel))
		level := 0
		orderedUnits := ordering.TimingRound(level)
		for orderedUnits != nil {
			for _, unit := range orderedUnits {
				units <- unit
			}
			level++
			orderedUnits = ordering.TimingRound(level)
		}
		close(units)
	}()
	return units
}

func getAllTimingUnits(poset gomel.Poset, pid uint16, generalConfig config.Configuration) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {

		// TODO types
		rs := random.NewTcSource(poset, int(pid))
		ordering := linear.NewOrdering(poset, rs, int(generalConfig.VotingLevel), int(generalConfig.PiDeltaLevel))
		level := 0
		timingUnit := ordering.DecideTimingOnLevel(level)
		for timingUnit != nil {
			units <- timingUnit
			level++
			timingUnit = ordering.DecideTimingOnLevel(level)
		}
		close(units)
	}()
	return units
}

func getMaximalUnitsSorted(poset gomel.Poset, pid uint16, generalConfig config.Configuration) chan gomel.Unit {
	units := make(chan gomel.Unit)
	go func() {
		poset.MaximalUnitsPerProcess().Iterate(func(forks []gomel.Unit) bool {
			// order of 'forks' list might be different for different posets depending on order in which units were add to it
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

// ComposeVerifiers composes provided verifiers into a single verifier. Created verifier fails immediately after it discovers a failure of one of
// its verifiers.
func ComposeVerifiers(verifiers ...PosetVerifier) PosetVerifier {
	return func(posets []gomel.Poset, pids []uint16, generalConfigs []config.Configuration) error {
		for _, verifier := range verifiers {
			if err := verifier(posets, pids, generalConfigs); err != nil {
				return err
			}
		}
		return nil
	}
}

// ComposeAdders composes provided 'adders' into a single 'adder'. Created 'adder' fails immediately after it discovers a
// failure of one of its 'adders'.
func ComposeAdders(adders ...AddingHandler) AddingHandler {

	return func(posets []gomel.Poset, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		for _, adder := range adders {
			if err := adder(posets, rss, preunit); err != nil {
				return err
			}
		}
		return nil
	}
}

func verifyUnitsUsingOrdering(ordering func(gomel.Poset, uint16, config.Configuration) chan gomel.Unit, checker func(u1, u2 gomel.Unit) error) PosetVerifier {
	return func(posets []gomel.Poset, pids []uint16, generalConfigs []config.Configuration) error {
		if len(posets) < 2 {
			return nil
		}
		var units1 []gomel.Unit
		for unit := range ordering(posets[0], pids[0], generalConfigs[0]) {
			units1 = append(units1, unit)
		}
		for ix, poset := range posets {
			units2 := ordering(poset, pids[ix], generalConfigs[ix])

			for _, unit1 := range units1 {
				unit2, open := <-units2

				if !open {
					return gomel.NewDataError(fmt.Sprintf("poset id=%d returned more units than poset id=%d", 0, ix))
				}

				if err := checker(unit1, unit2); err != nil {
					return err
				}
			}

			if _, open := <-units2; open {
				return gomel.NewDataError(fmt.Sprintf("poset id=%d returned more units than poset id=%d", ix, 0))
			}

		}
		return nil
	}
}

// VerifyTimingUnits returns a poset verifier that checks if all posets returns same set of timing units.
func VerifyTimingUnits() PosetVerifier {
	prevLevel := -1
	return verifyUnitsUsingOrdering(

		getAllTimingUnits,

		func(u1, u2 gomel.Unit) error {
			level := u1.Level()
			if level != prevLevel+1 {
				fmt.Println("broken ordering")
				// return gomel.NewDataError(
				// 	fmt.Sprintf("Missing timing unit for level %d - obtained %d. Unit: %+v", prevLevel+1, level, u1),
				// )
			}
			prevLevel = level

			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Posets selected different timing units")
			}
			return nil
		},
	)
}

// VerifyOrdering returns a PosetVerifier that compares if all posets orders their underlying units in the same way.
func VerifyOrdering() PosetVerifier {
	return verifyUnitsUsingOrdering(

		getOrderedUnits,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Posets differ in ordering")
			}
			return nil
		},
	)
}

// VerifyAllPosetsContainSameMaximalUnits returns a PosetVerifier that checks if all posets provide same set of maximal units.
func VerifyAllPosetsContainSameMaximalUnits() PosetVerifier {
	return verifyUnitsUsingOrdering(
		getMaximalUnitsSorted,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				fmt.Printf("u1 %+v\n", u1)
				fmt.Printf("u2 %+v\n", u2)
				return gomel.NewDataError("posets contains different maximal units")
			}
			return nil
		},
	)
}

// NewDefaultVerifier returns a PosetVerifier composed from VerifyAllPosetsContainSameMaximalUnits, VerifyTimingUnits and
// VerifyOrdering verifiers.
func NewDefaultVerifier() func([]gomel.Poset, []gomel.PrivateKey) PosetVerifier {
	return func(posets []gomel.Poset, privKeys []gomel.PrivateKey) PosetVerifier {
		return ComposeVerifiers(VerifyAllPosetsContainSameMaximalUnits(), VerifyTimingUnits(), VerifyOrdering())
	}
}

// NewNoOpVerifier returns a PosetVerifier that does not check provided posets and immediately answers that they are correct.
func NewNoOpVerifier() PosetVerifier {
	return func([]gomel.Poset, []uint16, []config.Configuration) error {
		fmt.Println("No verification step")
		return nil
	}
}

// Test is a helper function that performs a single test using provided TestingRoutineFactory.
func Test(
	pubKeys []gomel.PublicKey,
	privKeys []gomel.PrivateKey,
	testingRoutine TestingRoutine,
) error {

	nProcesses := len(pubKeys)
	posets := make([]gomel.Poset, 0, nProcesses)
	pids := make([]uint16, 0, nProcesses)
	generalConfig := config.NewDefaultConfiguration()
	configurations := make([]config.Configuration, 0, nProcesses)
	rss := make([]gomel.RandomSource, 0, nProcesses)

	for pid := uint16(0); len(posets) < nProcesses; pid++ {
		poset := growing.NewPoset(&gomel.PosetConfig{Keys: pubKeys})
		defer poset.Stop()
		posets = append(posets, poset)

		pids = append(pids, pid)
		configurations = append(configurations, generalConfig)
		rs := random.NewTcSource(poset, int(pid))
		rss = append(rss, rs)
	}

	unitCreator, addingHandler, verifier, stopCondition :=
		testingRoutine.CreateUnitCreator(posets, privKeys),
		testingRoutine.CreateAddingHandler(posets, privKeys, rss),
		testingRoutine.CreatePosetVerifier(posets, privKeys),
		testingRoutine.StopCondition()

	fmt.Println("Starting a testing routine")
	for !stopCondition(posets) {

		var newUnit gomel.Preunit
		var err error
		if newUnit, err = unitCreator(posets, privKeys, rss); err != nil {
			fmt.Fprintln(os.Stderr, "Unable to create a new unit")
			return err
		}

		// send the unit to all posets
		if err := addingHandler(posets, rss, newUnit); err != nil {
			fmt.Fprintln(os.Stderr, "Error while adding a unit to some poset:", err)
			return err
		}
	}
	fmt.Println("Testing routine finished")

	fmt.Println("Verification step")
	err := verifier(posets, pids, configurations)
	if err != nil {
		fmt.Println("Posets verfication failed", err.Error())
		return err
	}
	fmt.Println("Verification step finished")
	return nil
}
