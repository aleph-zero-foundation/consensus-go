package main

import (
	"fmt"
	"os"
	"strconv"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
)

type proc struct {
	publicKey  string
	privateKey string
	address    string
}

func makeProcess(i int) proc {
	pubKey, privKey, _ := signing.GenerateKeys()
	port := 21037 + i
	return proc{
		publicKey:  pubKey.Encode(),
		privateKey: privKey.Encode(),
		address:    "127.0.0.1:" + strconv.Itoa(port),
	}
}

func writeAll(f *os.File, processes []proc) {
	for _, p := range processes {
		f.WriteString(p.publicKey)
		f.WriteString(" ")
		f.WriteString(p.address)
		f.WriteString("\n")
	}
}

func writeFile(i int, processes []proc) {
	f, err := os.Create(strconv.Itoa(i) + ".keys")
	if err != nil {
		panic("oh boy")
	}
	defer f.Close()
	f.WriteString(processes[i].privateKey)
	f.WriteString(" ")
	f.WriteString(processes[i].address)
	f.WriteString("\n")
	writeAll(f, processes)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: gomel-keys <number>.\n")
		return
	}
	num, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Usage: gomel-keys <number>.\n")
		return
	}
	if num < 4 {
		fmt.Fprintf(os.Stderr, "Cannot have less than 4 processes.\n")
		return
	}
	processes := []proc{}
	for i := 0; i < num; i++ {
		processes = append(processes, makeProcess(i))
	}
	for i := range processes {
		writeFile(i, processes)
	}
}
