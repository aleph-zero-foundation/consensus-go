package signing

import gomel "gitlab.com/alephledger/consensus-go/pkg"

// PublicKey used for signature checking.
type PublicKey interface {
	// Verify checks if a preunit has a correct signature.
	Verify(gomel.Preunit) bool
}

// PrivateKey used for signing units.
type PrivateKey interface {
	// Sign computes and returns a signature of a preunit.
	Sign(gomel.Preunit) gomel.Signature
}
