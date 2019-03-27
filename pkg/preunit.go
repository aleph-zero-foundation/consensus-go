package alephzero

// A unit outside of a poset, so either just created or transferred through the network.
type Preunit interface {
	BaseUnit
	Parents() []Hash
}
