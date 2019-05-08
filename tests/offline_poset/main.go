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

	pubKeys := make([]signing.PublicKey, nProcesses, nProcesses)
	privKeys := make([]signing.PrivateKey, nProcesses, nProcesses)
	posets := make([]gomel.Poset, nProcesses, nProcesses)
	network := make([]chan gomel.Preunit, nProcesses, nProcesses)

	for pid := 0; pid < nProcesses; pid++ {
		pubKeys[pid], privKeys[pid], _ = signing.GenerateKeys()
	}

	var wg sync.WaitGroup

	// start goroutines waiting for a preunit and adding it to its' poset
	exit := make(chan struct{})
	for pid := 0; pid < nProcesses; pid++ {
		poset := growing.NewPoset(pubKeys)
		net := make(chan gomel.Preunit, nUnits/nProcesses)
		go func() {
			for {
				select {
				case pu := <-net:
					wg.Add(1)
					poset.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, err error) {
						wg.Done()
						if err != nil {
							fmt.Println(err)
						}
					})
				case <-exit:
					return
				}
			}
		}()
		posets[pid] = poset
		network[pid] = net
	}

	for i := 0; i < nUnits; i++ {
		// the following loop tries to create a one unit and after a success
		// it sends it to other posets and stops
		for {
			// choose the unit creator and create a unit
			creator := rand.Intn(nProcesses)
			poset := posets[creator]
			pu, err := creating.NewUnit(poset, creator, maxParents)
			if err != nil {
				continue
			}
			pu.SetSignature(privKeys[creator].Sign(pu))
			var wg sync.WaitGroup
			wg.Add(1)

			// add the unit to creator's poset
			if i%50 == 0 {
				fmt.Println("Addning unit no", i, "out of", nUnits)
			}
			poset.AddUnit(pu, func(pu gomel.Preunit, u gomel.Unit, err error) {
				wg.Done()
				if err != nil {
					fmt.Println(err, pu.Creator())
				}
			})
			wg.Wait()

			// send the unit to other posets
			for j := 0; j < nProcesses; j++ {
				if j == creator {
					continue
				}
				network[j] <- pu
			}
			break
		}
	}
	close(exit)
	wg.Wait()
}
