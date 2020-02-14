package gomel

import (
	"sync"

	"gitlab.com/alephledger/core-go/pkg/network"
)

// Alerter is responsible for raising alerts about forks and handling communication about commitments in case of fork.
type Alerter interface {
	Service
	// NewFork raises an alert about newly detected fork.
	NewFork(Preunit, Preunit)
	// HandleIncoming handles the incoming connection and signals the provided WaitGroup when done.
	HandleIncoming(network.Connection, *sync.WaitGroup)
	// Disambiguate which of the provided (forked) units is the right one to be the parent of the given preunit.
	Disambiguate([]Unit, Preunit) (Unit, error)
	// RequestCommitment that is missing in the given BaseUnit from the committee member with the given process ID.
	RequestCommitment(BaseUnit, uint16) error
	// ResolveMissingCommitment
	ResolveMissingCommitment(error, BaseUnit, uint16) error
	//IsForker checks whether the alerter knows that the given pid is a forker.
	IsForker(uint16) bool
	// Lock the state for the given process ID.
	Lock(uint16)
	// Unlock the state for the given process ID.
	Unlock(uint16)
}

// NopAlerter is an alerter that does nothing.
func NopAlerter() Alerter {
	return &nopAl{}
}

type nopAl struct{}

func (*nopAl) Start() error                                                 { return nil }
func (*nopAl) Stop()                                                        {}
func (*nopAl) NewFork(Preunit, Preunit)                                     {}
func (*nopAl) HandleIncoming(network.Connection, *sync.WaitGroup)           {}
func (*nopAl) Disambiguate([]Unit, Preunit) (Unit, error)                   { return nil, nil }
func (*nopAl) RequestCommitment(BaseUnit, uint16) error                     { return nil }
func (*nopAl) ResolveMissingCommitment(e error, _ BaseUnit, _ uint16) error { return e }
func (*nopAl) IsForker(uint16) bool                                         { return false }
func (*nopAl) Lock(uint16)                                                  {}
func (*nopAl) Unlock(uint16)                                                {}
