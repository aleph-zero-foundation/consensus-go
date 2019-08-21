package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/tests/offline_dag/helpers"
)

func runOfflineTest() {
	const (
		nProcesses = 50
		nUnits     = 5000
		maxParents = 10
	)

	pubKeys, privKeys := helpers.GenerateKeys(nProcesses)
	unitCreator := helpers.NewDefaultUnitCreator(helpers.NewDefaultCreator(maxParents))
	defaultAdder := helpers.NewDefaultAdder()
	unitAdder := func(dags []gomel.Dag, rss []gomel.RandomSource, preunit gomel.Preunit) error {
		err := defaultAdder(dags, rss, preunit)
		if err != nil {
			switch err.(type) {
			case *gomel.DuplicateUnit:
				fmt.Println(err)
			default:
				fmt.Println(err)
			}
		}
		return nil
	}
	createUnitAdder := func(dags []gomel.Dag, privKeys []gomel.PrivateKey, rss []gomel.RandomSource) helpers.AddingHandler {
		return unitAdder
	}
	verifier := helpers.NewDefaultVerifier()
	testingRoutine := helpers.NewDefaultTestingRoutine(
		func([]gomel.Dag, []gomel.PrivateKey) helpers.UnitCreator { return unitCreator },
		createUnitAdder,
		verifier,
	)

	if err := helpers.Test(pubKeys, privKeys, testingRoutine); err != nil {
		fmt.Println("test failed")
	}
}

var cpuprofile = flag.String("cpuprof", "", "the name of the file with cpu-profile results")
var memprofile = flag.String("memprof", "", "the name of the file with mem-profile results")

func main() {

	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating cpu-profile file \"%s\" failed because: %s.\n", cpuprofile, err.Error())
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Cpu-profile failed to start because: %s", err.Error())
		}
		defer pprof.StopCPUProfile()
	}

	runOfflineTest()

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Creating mem-profile file \"%s\" failed because: %s.\n", memprofile, err.Error())
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Mem-profile failed to start because: %s", err.Error())
		}
	}
}
