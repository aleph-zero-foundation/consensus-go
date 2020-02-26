package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/crypto/p2p"
)

// Member represents the private data about a committee member.
type Member struct {
	// The process id of this member.
	Pid uint16

	// The private key of this committee member.
	PrivateKey gomel.PrivateKey

	// The secret key of this committee member use for RMC.
	RMCSecretKey *bn256.SecretKey

	// The key for generating keys for p2p communication.
	P2PSecretKey *p2p.SecretKey
}

// Committee represents the public data about the committee known before the algorithm starts.
type Committee struct {
	// Public keys of all committee members, ordered according to process ids.
	PublicKeys []gomel.PublicKey

	// PublicKeys of all committee members use for generating keys for p2p communication. Ordered according to process ids.
	P2PPublicKeys []*p2p.PublicKey

	// Verification keys of all committee members use for RMC, ordered according to process ids.
	RMCVerificationKeys []*bn256.VerificationKey

	// SetupAddresses of all committee members
	SetupAddresses map[string][]string

	// Addresses of all committee members
	Addresses map[string][]string
}

const malformedData = "malformed committee data"

// LoadMember loads the data from the given reader and creates a member.
func LoadMember(r io.Reader) (*Member, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	// read private key, secret key, decryption key and pid. Assumes one line of the form
	// "key secret_key decryption_key pid"
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
	sKey, err := p2p.DecodeSecretKey(scanner.Text())
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
		P2PSecretKey: sKey,
	}, nil
}

func parseAddrs(addrsList string) map[string]string {
	addrs := make(map[string]string)
	for _, addr := range strings.Split(addrsList, " ") {
		if len(addr) == 0 {
			continue
		}
		switch addr[0] {
		case 'r':
			addrs["rmc"] = addr[1:]
		case 'f':
			addrs["fetch"] = addr[1:]
		case 'g':
			addrs["gossip"] = addr[1:]
		case 'm':
			addrs["mcast"] = addr[1:]
		}
	}
	return addrs
}

func parseCommitteeLine(line string) (string, string, string, map[string]string, map[string]string, error) {
	s := strings.Split(line, "|")

	if len(s) < 5 {
		return "", "", "", nil, nil, errors.New("commitee line should be of the form:\npublicKey|verifiactionKey|p2pPublicKey|setupAddresses|addresses")
	}
	pk, p2pPK, vk, setupAddrsList, addrsList := s[0], s[1], s[2], s[3], s[4]
	var errStrings []string
	if len(pk) == 0 {
		return "", "", "", nil, nil, errors.New(malformedData)
	}
	if len(p2pPK) == 0 {
		errStrings = append(errStrings, "p2p public key should be non-empty")
	}
	if len(vk) == 0 {
		errStrings = append(errStrings, "verification key should be non-empty")
	}
	if errStrings == nil {
		return pk, p2pPK, vk, parseAddrs(setupAddrsList), parseAddrs(addrsList), nil
	}
	return "", "", "", nil, nil, fmt.Errorf(strings.Join(errStrings, "\n"))
}

func addAddr(commAddrMap map[string][]string, addrMap map[string]string) {
	for syncType, addr := range addrMap {
		commAddrMap[syncType] = append(commAddrMap[syncType], addr)
	}
}

// LoadCommittee loads the data from the given reader and creates a committee.
func LoadCommittee(r io.Reader) (*Committee, error) {
	scanner := bufio.NewScanner(r)

	c := &Committee{SetupAddresses: make(map[string][]string), Addresses: make(map[string][]string)}
	for scanner.Scan() {
		pk, p2pPK, vk, setupAddrs, addrs, err := parseCommitteeLine(scanner.Text())
		if err != nil {
			return nil, err
		}

		publicKey, err := signing.DecodePublicKey(pk)
		if err != nil {
			return nil, err
		}

		p2pPublicKey, err := p2p.DecodePublicKey(p2pPK)
		if err != nil {
			return nil, err
		}

		verificationKey, err := bn256.DecodeVerificationKey(vk)
		if err != nil {
			return nil, err
		}

		c.PublicKeys = append(c.PublicKeys, publicKey)
		c.P2PPublicKeys = append(c.P2PPublicKeys, p2pPublicKey)
		c.RMCVerificationKeys = append(c.RMCVerificationKeys, verificationKey)
		addAddr(c.SetupAddresses, setupAddrs)
		addAddr(c.Addresses, addrs)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(c.PublicKeys) < 4 {
		return nil, errors.New(malformedData)
	}
	return c, nil
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
	_, err = io.WriteString(w, m.P2PSecretKey.Encode())
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

func storeAddresses(w io.Writer, pid int, addresses map[string][]string, types []string) error {
	for j, syncType := range types {
		if _, ok := addresses[syncType]; !ok {
			continue
		}
		if j != 0 {
			if _, err := io.WriteString(w, " "); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, syncType[0:1]); err != nil {
			return err
		}
		if _, err := io.WriteString(w, addresses[syncType][pid]); err != nil {
			return err
		}
	}
	return nil
}

// StoreCommittee writes the given committee to the writer.
func StoreCommittee(w io.Writer, c *Committee) error {
	for i := range c.PublicKeys {
		// store public keys
		if _, err := io.WriteString(w, c.PublicKeys[i].Encode()); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "|"); err != nil {
			return err
		}
		// store p2p keys
		if _, err := io.WriteString(w, c.P2PPublicKeys[i].Encode()); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "|"); err != nil {
			return err
		}
		// store verification keys for RMC
		if _, err := io.WriteString(w, c.RMCVerificationKeys[i].Encode()); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "|"); err != nil {
			return err
		}
		// store setup addresses
		if err := storeAddresses(w, i, c.SetupAddresses, []string{"rmc", "fetch", "gossip"}); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "|"); err != nil {
			return err
		}
		// store addresses
		if err := storeAddresses(w, i, c.Addresses, []string{"rmc", "mcast", "fetch", "gossip"}); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}
