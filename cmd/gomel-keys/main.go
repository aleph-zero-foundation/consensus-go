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
	publicKey  gomel.PublicKey
	privateKey gomel.PrivateKey
	address    string
	mcAddress  string
}

func makeProcess(address string, mcAddress string) proc {
	pubKey, privKey, _ := signing.GenerateKeys()
	return proc{
		publicKey:  pubKey,
		privateKey: privKey,
		address:    address,
		mcAddress:  mcAddress,
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
	num, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, usageMsg)
		return
	}
	if num < 4 {
		fmt.Fprintln(os.Stderr, "Cannot have less than 4 processes.")
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Usage: gomel-keys <number> [<addresses_file>].")
		return
	}
	addresses := []string{}
	mcAddresses := []string{}
	if len(os.Args) == 2 {
		for i := 0; i < num; i++ {
			addresses = append(addresses, "127.0.0.1:"+strconv.Itoa(9000+i))
			mcAddresses = append(mcAddresses, "127.0.0.1:"+strconv.Itoa(10000+i))
		}
	} else {
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Cannot open file ", os.Args[2])
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			addr := scanner.Text()
			i := strings.Index(addr, ":")
			j := strings.Index(addr[i+1:], ":")
			ip := addr[:i]
			port := addr[i : i+j+1]
			mcPort := addr[i+j+1:]
			addresses = append(addresses, ip+port)
			mcAddresses = append(mcAddresses, ip+mcPort)
		}
		if len(addresses) < num {
			fmt.Fprintln(os.Stderr, "Too few addresses in ", os.Args[2])
			return
		}
	}
	processes := []proc{}
	for i := 0; i < num; i++ {
		processes = append(processes, makeProcess(addresses[i], mcAddresses[i]))
	}
	committee := &config.Committee{}
	for _, p := range processes {
		committee.PublicKeys = append(committee.PublicKeys, p.publicKey)
		committee.Addresses = append(committee.Addresses, p.address)
		committee.MCAddresses = append(committee.MCAddresses, p.mcAddress)
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
