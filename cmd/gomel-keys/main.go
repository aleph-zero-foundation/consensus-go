package main

import (
	"fmt"
	"os"
	"strconv"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
)

type proc struct {
	publicKey  gomel.PublicKey
	privateKey gomel.PrivateKey
	address    string
}

func makeProcess(i int) proc {
	pubKey, privKey, _ := signing.GenerateKeys()
	port := 21037 + i
	return proc{
		publicKey:  pubKey,
		privateKey: privKey,
		address:    "127.0.0.1:" + strconv.Itoa(port),
	}
}

// This program generates files with random keys and local addresses for a committee of the specified size.
// These files are intended to be used for simple local tests of the gomel binary.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: gomel-keys <number>.")
		return
	}
	num, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Usage: gomel-keys <number>.")
		return
	}
	if num < 4 {
		fmt.Fprintln(os.Stderr, "Cannot have less than 4 processes.")
		return
	}
	processes := []proc{}
	for i := 0; i < num; i++ {
		processes = append(processes, makeProcess(i))
	}
	committee := &config.Committee{}
	for _, p := range processes {
		committee.PublicKeys = append(committee.PublicKeys, p.publicKey)
		committee.Addresses = append(committee.Addresses, p.address)
	}
	for i, p := range processes {
		f, err := os.Create(strconv.Itoa(i) + ".keys")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		defer f.Close()
		committee.Pid = i
		committee.PrivateKey = p.privateKey
		err = config.StoreCommittee(f, committee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
	}
}
