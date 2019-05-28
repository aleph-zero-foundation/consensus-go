package gomel

import "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

// Preunit represents a unit which does not (yet) belong to a poset, so either just created or transferred through the network.
type Preunit interface {
	BaseUnit
	// Parents of a preunit are identified by their hashes, since preunits exist outside of a poset.
	Parents() []Hash
	// SetSignature sets a signature of this preunit.
	SetSignature(Signature)
	// GlobalThresholdCoin is a threshold coin dealt by this preunit
	// it is nil for non-dealing units
	GlobalThresholdCoin() *tcoin.GlobalThresholdCoin
}
