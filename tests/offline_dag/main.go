package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/random/urn"
)

func runOfflineTest() {
	// size of the test
	nProcesses := 50
	maxParents := 10
	nUnits := 5000

	pubKeys := make([]gomel.PublicKey, nProcesses)
	privKeys := make([]gomel.PrivateKey, nProcesses)

	for pid := 0; pid < nProcesses; pid++ {
		pubKeys[pid], privKeys[pid], _ = signing.GenerateKeys()
	}

	config := &gomel.DagConfig{pubKeys}
	dags := make([]gomel.Dag, nProcesses)
	rses := make([]gomel.RandomSource, nProcesses)

	// start goroutines waiting for a preunit and adding it to its' dag
	for pid := 0; pid < nProcesses; pid++ {
		dags[pid] = growing.NewDag(config)
		rses[pid] = urn.NewUrn(pid)
		rses[pid].Init(dags[pid])
	}

	for i := 0; i < nUnits; i++ {
		// the following loop tries to create a one unit and after a success
		// it sends it to other dags and stops
		var pu gomel.Preunit
		for {
			// choose the unit creator and create a unit
			creator := rand.Intn(nProcesses)
			dag := dags[creator]
			rs := rses[creator]
			var err error
			if pu, err = creating.NewUnit(dag, creator, maxParents, []byte{}, rs, true); err != nil {
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))

			// add the unit to creator's dag
			if i%50 == 0 {
				fmt.Println("Adding unit no", i, "out of", nUnits)
			}
			break
		}

		// send the unit to other dags
		var wg sync.WaitGroup
		wg.Add(nProcesses)
		for j := 0; j < nProcesses; j++ {
			dags[j].AddUnit(pu, rses[j], func(pu gomel.Preunit, u gomel.Unit, err error) {
				defer wg.Done()
				if err != nil {
					switch err.(type) {
					case *gomel.DuplicateUnit:
						fmt.Println(err)
					default:
						fmt.Println(err)
					}
				}
			})
		}
		wg.Wait()
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
