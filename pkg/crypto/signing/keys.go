package signing

import gomel "gitlab.com/alephledger/consensus-go/pkg"

// PublicKey used for signature checking
type PublicKey interface {
	Verify(gomel.Preunit) bool
}

// PrivateKey used for signing units
type PrivateKey interface {
	Sign(gomel.Preunit) gomel.Signature
}
