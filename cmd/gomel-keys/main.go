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
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
)

type memberKeys struct {
	publicKey  gomel.PublicKey
	privateKey gomel.PrivateKey
	sekKey     *bn256.SecretKey
	verKey     *bn256.VerificationKey
	p2pPubKey  *p2p.PublicKey
	p2pSecKey  *p2p.SecretKey
	addresses  map[string][]string
}

func makeMemberKeys(addresses map[string][]string) memberKeys {
	pubKey, privKey, _ := signing.GenerateKeys()
	verKey, sekKey, _ := bn256.GenerateKeys()
	p2pPubKey, p2pSecKey, _ := p2p.GenerateKeys()

	return memberKeys{
		publicKey:  pubKey,
		privateKey: privKey,
		sekKey:     sekKey,
		verKey:     verKey,
		p2pPubKey:  p2pPubKey,
		p2pSecKey:  p2pSecKey,
		addresses:  addresses,
	}
}

// This program generates files with random keys and local addresses for a committee of the specified size.
// These files are intended to be used for local and AWS tests of the gomel binary.
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
		fmt.Fprintln(os.Stderr, "Cannot have less than 4 keys.")
		return
	}

	addresses := make(map[string][]string)
	if len(os.Args) == 2 {
		for i := 0; i < nProc; i++ {
			addresses["rmc"] = append(addresses["rmc"], "127.0.0.1:"+strconv.Itoa(9000+i))
			addresses["mcast"] = append(addresses["mcast"], "127.0.0.1:"+strconv.Itoa(10000+i))
			addresses["fetch"] = append(addresses["fetch"], "127.0.0.1:"+strconv.Itoa(11000+i))
			addresses["gossip"] = append(addresses["gossip"], "127.0.0.1:"+strconv.Itoa(12000+i))
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
			for _, addr := range strings.Split(scanner.Text(), " ") {
				switch addr[0] {
				case 'r':
					addresses["rmc"] = append(addresses["rmc"], addr[1:])
				case 'm':
					addresses["mcast"] = append(addresses["mcast"], addr[1:])
				case 'f':
					addresses["fetch"] = append(addresses["fetch"], addr[1:])
				case 'g':
					addresses["gossip"] = append(addresses["gossip"], addr[1:])
				}
			}
		}
	}
	keys := []memberKeys{}
	for pid := 0; pid < nProc; pid++ {
		keys = append(keys, makeMemberKeys(addresses))
	}
	committee := &config.Committee{}
	committee.Addresses = addresses
	for _, ks := range keys {
		committee.PublicKeys = append(committee.PublicKeys, ks.publicKey)
		committee.RMCVerificationKeys = append(committee.RMCVerificationKeys, ks.verKey)
		committee.P2PPublicKeys = append(committee.P2PPublicKeys, ks.p2pPubKey)
	}

	for pid, ks := range keys {
		member := &config.Member{uint16(pid), ks.privateKey, ks.sekKey, ks.p2pSecKey}
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
