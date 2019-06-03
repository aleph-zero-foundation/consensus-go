package offline_poset

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
)

const (
	maxParents = 2
)

// UnitCreator is a type of a function that given a list of posets should create a new unit or return an error otherwise
type UnitCreator func([]*growing.Poset) (gomel.Preunit, error)

// AddingHandler is a type of a function that given a list of posets and a unit is supposed appropriately handle addition of
// that unit with accordance to used strategy
type AddingHandler func(posets []*growing.Poset, unit gomel.Preunit) error

// PosetVerifier is a type of a function that is responsible of verifying if a given list of posests is valid after performed
// test
type PosetVerifier func([]*growing.Poset) error

// TestingRoutine describes a strategy for performing a test on a given set of posets
type TestingRoutine interface {
	CreateUnitCreator() UnitCreator
	CreateAddingHandler() AddingHandler
	CreatePosetVerifier() PosetVerifier
}

// TestingRoutineFactory represent a particular test executed on a given list of posets
type TestingRoutineFactory func([]*growing.Poset) ([]*growing.Poset, TestingRoutine)

type testingRoutineFactory struct {
	posetInitializer func([]*growing.Poset) []*growing.Poset
	creator          UnitCreator
	adder            AddingHandler
	verifier         PosetVerifier
}

func (test *testingRoutineFactory) CreateUnitCreator() UnitCreator {
	return test.creator
}

func (test *testingRoutineFactory) CreateAddingHandler() AddingHandler {
	return test.adder
}

func (test *testingRoutineFactory) CreatePosetVerifier() PosetVerifier {
	return test.verifier
}

// NewDefaultTestingRoutineFactory creates an instance of TestingRoutine
func NewDefaultTestingRoutineFactory(posetInitializer func([]*growing.Poset) []*growing.Poset, creator UnitCreator, adder AddingHandler, verifier PosetVerifier) TestingRoutineFactory {
	return func(posets []*growing.Poset) ([]*growing.Poset, TestingRoutine) {
		return posetInitializer(posets), &testingRoutineFactory{posetInitializer, creator, adder, verifier}
	}
}

// NewDefaultAdder creates an instance of AddingHandler that simply ads a given unit to all posets under test
func NewDefaultAdder() AddingHandler {
	return func(posets []*growing.Poset, unit gomel.Preunit) error {
		_, err := AddToPosets(unit, posets)
		return err
	}
}

// AddToPoset is a helper method for synchronous addition of a unit to a given poset
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

// AddToPosets is a helper function that adds a given unit to all provided posets
func AddToPosets(unit gomel.Preunit, posets []*growing.Poset) (resultUnit gomel.Unit, err error) {
	for ix, poset := range posets {
		result, errTmp := AddToPoset(poset, unit)
		if errTmp != nil {
			err = errTmp
			return
		}
		if ix == unit.Creator() {
			resultUnit = result
		}
	}
	return
}

// AddUnitsToPosetsInRandomOrder adds a set of units in random order (per each poset) to all provided posets
func AddUnitsToPosetsInRandomOrder(units []gomel.Preunit, posets []*growing.Poset) error {
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

// GenerateKeys is a helper function creating a set of pairs of public-private keys
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
// poset
func NewDefaultUnitCreator(privKeys []gomel.PrivateKey) UnitCreator {
	return func(posets []*growing.Poset) (gomel.Preunit, error) {
		attempts := 0
		for {
			attempts++
			if attempts%50 == 0 {
				fmt.Println("Attempt no", attempts, "of creating a new unit")
			}

			creator := rand.Intn(len(posets))
			poset := posets[creator]

			pu, err := creating.NewUnit(poset, creator, maxParents, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error while creating a new unit:", err)
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))
			return pu, nil
		}
	}
}

// GetOrderedUnits returns units of a given poset using default implementation of the ordering interface
func GetOrderedUnits(poset gomel.Poset) (units chan gomel.Unit) {
	units = make(chan gomel.Unit)
	go func() {
		config := config.NewDefaultConfiguration()
		// TODO types
		ordering := linear.NewOrdering(poset, int(config.VotingLevel), int(config.PiDeltaLevel))
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

// GetAllTimingUnits returns all timing units of a poset in increasing order of their levels
func GetAllTimingUnits(poset gomel.Poset) (units chan gomel.Unit) {
	units = make(chan gomel.Unit)
	go func() {
		config := config.NewDefaultConfiguration()
		// TODO types
		ordering := linear.NewOrdering(poset, int(config.VotingLevel), int(config.PiDeltaLevel))
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

// GetMaximalUnitsSorted returns a list of maximal units of a poset. Forks are sorted using their Hash values
func GetMaximalUnitsSorted(poset gomel.Poset) (units chan gomel.Unit) {
	units = make(chan gomel.Unit)
	go func() {
		poset.MaximalUnitsPerProcess().Iterate(func(forks []gomel.Unit) bool {
			sorted := make([]gomel.Unit, len(forks))
			copy(sorted, forks)
			sort.Slice(sorted, func(i, j int) bool {
				a, b := sorted[i], sorted[j]
				return a.Hash().LessThan(b.Hash())
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

// ComposeVerifiers composes provided verifiers into a single one. It fails immediately after it discovers a failure of one of
// the verifiers
func ComposeVerifiers(verifiers ...PosetVerifier) PosetVerifier {
	return func(posets []*growing.Poset) error {
		for _, verifier := range verifiers {
			if err := verifier(posets); err != nil {
				return err
			}
		}
		return nil
	}
}

func verifyUnitsUsingOrdering(ordering func(gomel.Poset) chan gomel.Unit, checker func(u1, u2 gomel.Unit) error) PosetVerifier {
	return func(posets []*growing.Poset) error {
		if len(posets) < 2 {
			return nil
		}
		for ix := range posets[:len(posets)-1] {
			fmt.Println("verifying poset no", ix)
			units1 := ordering(posets[ix])
			units2 := ordering(posets[ix+1])

			for unit1 := range units1 {
				unit2, open := <-units2

				if !open {
					return gomel.NewDataError("Posets contains different number of units")
				}

				if err := checker(unit1, unit2); err != nil {
					return err
				}
			}

			if _, open := <-units2; open {
				return gomel.NewDataError("Posets returned different number of units")
			}

		}
		return nil
	}
}

// VerifyTimingUnits returns a poset verifier that checks if all posets returns same set of timing units
func VerifyTimingUnits() PosetVerifier {
	prevLevel := -1
	return verifyUnitsUsingOrdering(

		GetAllTimingUnits,

		func(u1, u2 gomel.Unit) error {
			level := u1.Level()
			if level != prevLevel+1 {
				// TODO
				return gomel.NewDataError(
					fmt.Sprintf("Missing timing unit for level %d - obtained %d. Unit: %+v", prevLevel+1, level, u1),
				)
			}
			prevLevel = level

			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Posets selected different timing units")
			}
			return nil
		},
	)
}

// VerifyOrdering returns a PosetVerifier that compares if all posets orders their underlying units in the same way
func VerifyOrdering() PosetVerifier {
	return verifyUnitsUsingOrdering(

		GetOrderedUnits,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("Posets differ in ordering")
			}
			return nil
		},
	)
}

// VerifyAllPosetsContainSameMaximalUnits returns a PosetVerifier that checks if all posets provide same set of maximal units
func VerifyAllPosetsContainSameMaximalUnits() PosetVerifier {
	return verifyUnitsUsingOrdering(
		GetMaximalUnitsSorted,

		func(u1, u2 gomel.Unit) error {
			if *u1.Hash() != *u2.Hash() {
				return gomel.NewDataError("posets contains different maximal units")
			}
			return nil
		},
	)
}

// NewDefaultVerifier returns default PosetVerifier composed from VerifyAllPosetsContainSameMaximalUnits, VerifyTimingUnits and
// VerifyOrdering verifiers
func NewDefaultVerifier() PosetVerifier {
	return ComposeVerifiers(VerifyAllPosetsContainSameMaximalUnits(), VerifyTimingUnits(), VerifyOrdering())
}

// NewNoOpVerifier returns a PosetVerifier that does not check provided posets and simply answer that they are correct
func NewNoOpVerifier() PosetVerifier {
	return func([]*growing.Poset) error {
		fmt.Println("No verification step")
		return nil
	}
}

// Test is a helper function that performs a test using provided TestingRoutineFactory
func Test(
	pubKeys []gomel.PublicKey,
	nUnits, maxParents int,
	testRoutineFactory TestingRoutineFactory,
) error {

	nProcesses := len(pubKeys)
	posets := make([]*growing.Poset, 0, nProcesses)

	for len(posets) < nProcesses {
		posets = append(posets, growing.NewPoset(&gomel.PosetConfig{Keys: pubKeys}))
	}

	var testingRoutine TestingRoutine
	posets, testingRoutine = testRoutineFactory(posets)
	unitCreator, addingHandler, verifier :=
		testingRoutine.CreateUnitCreator(),
		testingRoutine.CreateAddingHandler(),
		testingRoutine.CreatePosetVerifier()

	fmt.Println("Starting a testing routine")
	for u := 0; u < nUnits; u++ {

		var newUnit gomel.Preunit
		var err error
		if newUnit, err = unitCreator(posets); err != nil {
			fmt.Fprintln(os.Stderr, "Unable to create a new unit")
			return err
		}

		// send the unit to all posets
		if err := addingHandler(posets, newUnit); err != nil {
			fmt.Fprintln(os.Stderr, "Error while adding a unit to some poset:", err)
			return err
		}
	}
	fmt.Println("Testing routine finished")

	fmt.Println("Verification step")
	err := verifier(posets)
	if err != nil {
		fmt.Println("Posets verfication failed", err.Error())
		return err
	}
	fmt.Println("Verification step finished")
	return nil
}
