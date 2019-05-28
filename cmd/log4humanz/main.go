package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "log4humanz: I need a file with JSON log as a single argument")
		return
	}

	file, err := os.Open(flag.Args()[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: file not present\n", flag.Args()[0])
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	decoder := logging.NewDecoder(os.Stdout)
	for scanner.Scan() {
		decoder.Write([]byte(scanner.Text()))
	}

}
