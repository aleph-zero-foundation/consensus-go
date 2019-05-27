package main

import (
	"fmt"
	"math/rand"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
)

func main() {
	// size of the test
	nProcesses := 4
	maxParents := 2
	nUnits := 500

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
			if pu, err = creating.NewUnit(poset, creator, maxParents, []byte{}); err != nil {
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
