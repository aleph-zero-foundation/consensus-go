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
		fmt.Fprintln(os.Stderr, "Usage: log4humanz logfile.json")
		return
	}

	var file *os.File
	switch _, err := os.Stat(flag.Args()[0]); {
	case os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "%s: file not present\n", flag.Args()[0])
		return
	case err != nil:
		fmt.Fprintf(os.Stderr, "%s: cannot open file\n", flag.Args()[0])
		return
	default:
		file, err = os.Open(flag.Args()[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot open file\n", flag.Args()[0])
			return
		}
		defer file.Close()
	}

	scanner := bufio.NewScanner(file)
	decoder := logging.NewDecoder(os.Stdout)
	for scanner.Scan() {
		decoder.Write([]byte(scanner.Text()))
	}

}
