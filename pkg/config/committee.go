package config

import (
	"bufio"
	"errors"
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Committee represents the public data about the committee known before the algorithm starts.
type Committee struct {
	// The process id of this member.
	Pid int

	// The private key of this committee member.
	PrivateKey gomel.PrivateKey

	// Public keys of all committee members, ordered according to process ids.
	PublicKeys []gomel.PublicKey

	// Addresses of all committee members, ordered according to process ids.
	Addresses []string

	// Addresses use for multicast, ordered as above.
	MCAddresses []string

	// Addresses use for the setup phase, ordered as above.
	SetupAddresses []string

	// Addresses use for multicast in the setup phase, ordered as above
	SetupMCAddresses []string
}

const malformedData = "malformed committee data"

// LoadCommittee loads the data from the given reader and creates a committee.
func LoadCommittee(r io.Reader) (*Committee, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	if !scanner.Scan() {
		return nil, errors.New(malformedData)
	}
	privateKey, err := signing.DecodePrivateKey(scanner.Text())
	if err != nil {
		return nil, err
	}
	if !scanner.Scan() {
		return nil, errors.New(malformedData)
	}
	address := scanner.Text()
	publicKeys := []gomel.PublicKey{}
	remoteAddresses := []string{}
	MCAddresses := []string{}
	setupAddresses := []string{}
	setupMCAddresses := []string{}
	for scanner.Scan() {
		publicKey, err := signing.DecodePublicKey(scanner.Text())
		if err != nil {
			return nil, err
		}
		publicKeys = append(publicKeys, publicKey)
		if !scanner.Scan() {
			return nil, errors.New(malformedData)
		}
		remoteAddresses = append(remoteAddresses, scanner.Text())
		if !scanner.Scan() {
			return nil, errors.New(malformedData)
		}
		MCAddresses = append(MCAddresses, scanner.Text())
		if !scanner.Scan() {
			return nil, errors.New(malformedData)
		}
		setupAddresses = append(setupAddresses, scanner.Text())
		if !scanner.Scan() {
			return nil, errors.New(malformedData)
		}
		setupMCAddresses = append(setupMCAddresses, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(publicKeys) < 4 {
		return nil, errors.New(malformedData)
	}
	pid := -1
	for i, a := range remoteAddresses {
		if a == address {
			pid = i
			break
		}
	}
	if pid == -1 {
		return nil, errors.New(malformedData)
	}
	return &Committee{
		Pid:              pid,
		PrivateKey:       privateKey,
		PublicKeys:       publicKeys,
		Addresses:        remoteAddresses,
		MCAddresses:      MCAddresses,
		SetupAddresses:   setupAddresses,
		SetupMCAddresses: setupMCAddresses,
	}, nil
}

// StoreCommittee writes the given committee to the writer.
func StoreCommittee(w io.Writer, c *Committee) error {
	_, err := io.WriteString(w, c.PrivateKey.Encode())
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, " ")
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, c.Addresses[c.Pid])
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	if err != nil {
		return err
	}
	for i := range c.Addresses {
		_, err = io.WriteString(w, c.PublicKeys[i].Encode())
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, " ")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, c.Addresses[i])
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, " ")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, c.MCAddresses[i])
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, " ")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, c.SetupAddresses[i])
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, " ")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, c.SetupMCAddresses[i])
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "\n")
		if err != nil {
			return err
		}
	}
	return nil
}
