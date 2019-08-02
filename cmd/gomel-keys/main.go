package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type proc struct {
	publicKey       gomel.PublicKey
	privateKey      gomel.PrivateKey
	localAddrs      []string
	setupLocalAddrs []string
}

func makeProcess(setupLocalAddrs []string, localAddrs []string) proc {
	pubKey, privKey, _ := signing.GenerateKeys()
	return proc{
		publicKey:       pubKey,
		privateKey:      privKey,
		setupLocalAddrs: setupLocalAddrs,
		localAddrs:      localAddrs,
	}
}

// This program generates files with random keys and local addresses for a committee of the specified size.
// These files are intended to be used for simple local tests of the gomel binary.
func main() {
	usageMsg := "Usage: gomel-keys <number> [<addresses_file>]."
	if len(os.Args) != 2 && len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, usageMsg)
		return
	}
	nProc, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, usageMsg)
		return
	}
	if nProc < 4 {
		fmt.Fprintln(os.Stderr, "Cannot have less than 4 processes.")
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, usageMsg)
		return
	}
	// addresses for gossip and multicast
	setupAddresses := make([][]string, nProc)
	addresses := make([][]string, nProc)
	if len(os.Args) == 2 {
		for i := 0; i < nProc; i++ {
			// gossip
			setupAddresses[i] = append(setupAddresses[i], "127.0.0.1:"+strconv.Itoa(11000+i))
			// multicast
			setupAddresses[i] = append(setupAddresses[i], "127.0.0.1:"+strconv.Itoa(12000+i))
			// gossip
			addresses[i] = append(addresses[i], "127.0.0.1:"+strconv.Itoa(9000+i))
			// multicast
			addresses[i] = append(addresses[i], "127.0.0.1:"+strconv.Itoa(10000+i))
		}
	} else {
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot open file ", os.Args[2])
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for pid := 0; pid < nProc && scanner.Scan(); pid++ {
			s := strings.Split(scanner.Text(), "|")
			setupAddrs, addrs := s[0], s[1]
			for _, addr := range strings.Split(setupAddrs, " ") {
				setupAddresses[pid] = append(setupAddresses[pid], addr)
			}
			for _, addr := range strings.Split(addrs, " ") {
				addresses[pid] = append(addresses[pid], addr)
			}
		}
	}
	processes := []proc{}
	for i := 0; i < nProc; i++ {
		processes = append(processes, makeProcess(setupAddresses[i], addresses[i]))
	}
	committee := &config.Committee{}
	committee.SetupAddresses = make([][]string, len(setupAddresses[0]))
	committee.Addresses = make([][]string, len(addresses[0]))
	for _, p := range processes {
		committee.PublicKeys = append(committee.PublicKeys, p.publicKey)
		for i, addr := range p.setupLocalAddrs {
			committee.SetupAddresses[i] = append(committee.SetupAddresses[i], addr)
		}
		for i, addr := range p.localAddrs {
			committee.Addresses[i] = append(committee.Addresses[i], addr)
		}
	}

	for pid, p := range processes {
		member := &config.Member{pid, p.privateKey}
		f, err := os.Create(strconv.Itoa(pid) + ".pk")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		defer f.Close()
		err = config.StoreMember(f, member)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}

	}
	f, err := os.Create("committee.ka")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	defer f.Close()
	err = config.StoreCommittee(f, committee)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

}
