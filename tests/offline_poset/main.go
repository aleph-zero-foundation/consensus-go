package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
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

	config := &gomel.PosetConfig{pubKeys}
	posets := make([]gomel.Poset, nProcesses)

	// start goroutines waiting for a preunit and adding it to its' poset
	for pid := 0; pid < nProcesses; pid++ {
		posets[pid] = growing.NewPoset(config)
	}

	for i := 0; i < nUnits; i++ {
		// the following loop tries to create a one unit and after a success
		// it sends it to other posets and stops
		var pu gomel.Preunit
		for {
			// choose the unit creator and create a unit
			creator := rand.Intn(nProcesses)
			poset := posets[creator]
			var err error
			if pu, err = creating.NewUnit(poset, creator, maxParents, []byte{}, true); err != nil {
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))

			// add the unit to creator's poset
			if i%50 == 0 {
				fmt.Println("Adding unit no", i, "out of", nUnits)
			}
			break
		}

		// send the unit to other posets
		var wg sync.WaitGroup
		wg.Add(nProcesses)
		for j := 0; j < nProcesses; j++ {
			posets[j].AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, err error) {
				defer wg.Done()
				if err != nil {
					fmt.Println(err)
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
