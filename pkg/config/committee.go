package config

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Member represents the private data about a committee member.
type Member struct {
	// The process id of this member.
	Pid uint16

	// The private key of this committee member.
	PrivateKey gomel.PrivateKey

	// The secret key of this committee member use for RMC.
	RMCSecretKey *bn256.SecretKey
}

// Committee represents the public data about the committee known before the algorithm starts.
type Committee struct {
	// Public keys of all committee members, ordered according to process ids.
	PublicKeys []gomel.PublicKey

	// Verification keys of all committee members use for RMC, ordered according to process ids.
	RMCVerificationKeys []*bn256.VerificationKey

	// Addresses use for the setup phase, ordered as above.
	SetupAddresses [][]string

	// Addresses of all committee members, gathered in a list for all type of services in use and
	// every entry in that list is ordered according to process ids.
	Addresses [][]string
}

const malformedData = "malformed committee data"

// LoadMember loads the data from the given reader and creates a member.
func LoadMember(r io.Reader) (*Member, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	// read private key, secret key and pid. Assumes one line of the form
	// "key secret_key pid"
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
	secretKey, err := bn256.DecodeSecretKey(scanner.Text())
	if err != nil {
		return nil, err
	}
	if !scanner.Scan() {
		return nil, errors.New(malformedData)
	}
	pid, err := strconv.Atoi(scanner.Text())
	if err != nil {
		return nil, err
	}

	return &Member{
		Pid:          uint16(pid),
		RMCSecretKey: secretKey,
		PrivateKey:   privateKey,
	}, nil
}

func parseCommitteeLine(line string) (string, string, []string, []string, error) {
	s := strings.Split(line, "|")
	if len(s) < 4 {
		return "", "", nil, nil, errors.New(malformedData)
	}
	pk, vk, setupAddrs, addrs := s[0], s[1], s[2], s[3]
	if len(pk) == 0 {
		return "", "", nil, nil, errors.New(malformedData)
	}
	if len(vk) == 0 {
		return pk, "", nil, nil, errors.New(malformedData)
	}
	if len(addrs) == 0 {
		return pk, vk, nil, nil, errors.New(malformedData)
	}
	if len(setupAddrs) == 0 {
		return pk, vk, nil, strings.Split(addrs, " "), nil
	}
	return pk, vk, strings.Split(setupAddrs, " "), strings.Split(addrs, " "), nil
}

// LoadCommittee loads the data from the given reader and creates a committee.
func LoadCommittee(r io.Reader) (*Committee, error) {
	scanner := bufio.NewScanner(r)

	publicKeys := []gomel.PublicKey{}
	verificationKeys := []*bn256.VerificationKey{}
	sRemoteAddresses := [][]string{}
	remoteAddresses := [][]string{}
	for scanner.Scan() {
		pk, vk, setupAddresses, addresses, err := parseCommitteeLine(scanner.Text())
		if err != nil {
			return nil, err
		}

		publicKey, err := signing.DecodePublicKey(pk)
		if err != nil {
			return nil, err
		}

		verificationKey, err := bn256.DecodeVerificationKey(vk)
		if err != nil {
			return nil, err
		}

		publicKeys = append(publicKeys, publicKey)
		verificationKeys = append(verificationKeys, verificationKey)
		if len(remoteAddresses) == 0 {
			sRemoteAddresses = make([][]string, len(setupAddresses))
			remoteAddresses = make([][]string, len(addresses))
		}
		for i, address := range setupAddresses {
			sRemoteAddresses[i] = append(sRemoteAddresses[i], address)
		}
		for i, address := range addresses {
			remoteAddresses[i] = append(remoteAddresses[i], address)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(publicKeys) < 4 {
		return nil, errors.New(malformedData)
	}
	return &Committee{
		PublicKeys:          publicKeys,
		RMCVerificationKeys: verificationKeys,
		Addresses:           remoteAddresses,
		SetupAddresses:      sRemoteAddresses,
	}, nil
}

// StoreMember writes the given member to the writer.
func StoreMember(w io.Writer, m *Member) error {
	_, err := io.WriteString(w, m.PrivateKey.Encode())
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, " ")
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, m.RMCSecretKey.Encode())
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, " ")
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, strconv.Itoa(int(m.Pid)))
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	if err != nil {
		return err
	}
	return nil
}

func store(w io.Writer, addresses [][]string, i int) error {
	_, err := io.WriteString(w, "|")
	if err != nil {
		return err
	}
	for j := range addresses {
		if j != 0 {
			_, err = io.WriteString(w, " ")
			if err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, addresses[j][i])
		if err != nil {
			return err
		}
	}
	return nil
}

// StoreCommittee writes the given committee to the writer.
func StoreCommittee(w io.Writer, c *Committee) error {
	for i, pk := range c.PublicKeys {
		_, err := io.WriteString(w, pk.Encode())
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "|")
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, c.RMCVerificationKeys[i].Encode())
		if err != nil {
			return err
		}
		err = store(w, c.SetupAddresses, i)
		if err != nil {
			return err
		}
		err = store(w, c.Addresses, i)
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
