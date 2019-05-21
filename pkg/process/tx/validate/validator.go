package validate

import (
	"bufio"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"os"
)

type validator struct {
	userBalance map[string]uint32
}

func readUserBalance(filename string) (map[string]uint32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	userBalance := make(map[string]uint32)
	for scanner.Scan() {
		// Everyone starts with 10000, we can move start balance to the test file
		userBalance[scanner.Text()] = uint32(10000)
	}
	return userBalance, nil
}

func newValidator(userDb string) (*validator, error) {
	userBalance, err := readUserBalance(userDb)
	if err != nil {
		return nil, err
	}
	return &validator{userBalance: userBalance}, nil
}

func (v *validator) validate(tx gomel.Tx) {
	if v.userBalance[tx.Issuer] >= tx.Amount {
		v.userBalance[tx.Issuer] -= tx.Amount
		v.userBalance[tx.Receiver] += tx.Amount
	}
}
