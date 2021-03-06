package gomel

import (
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/utils"
)

// Alerter is responsible for raising alerts about forks and handling communication about commitments in case of fork.
type Alerter interface {
	// NewFork raises an alert about newly detected fork.
	NewFork(Preunit, Preunit)
	// HandleIncoming handles the incoming connection.
	HandleIncoming(network.Connection)
	// Disambiguate which of the provided (forked) units is the right one to be the parent of the given preunit.
	Disambiguate([]Unit, Preunit) (Unit, error)
	// RequestCommitment that is missing in the given Preunit from the committee member with the given process ID.
	RequestCommitment(Preunit, uint16) error
	// ResolveMissingCommitment
	ResolveMissingCommitment(error, Preunit, uint16) error
	//IsForker checks whether the alerter knows that the given pid is a forker.
	IsForker(uint16) bool
	// AddForkObserver allows one to receive notifications in case a fork is discovered.
	AddForkObserver(func(Preunit, Preunit)) utils.ObserverManager
	// Lock the state for the given process ID.
	Lock(uint16)
	// Unlock the state for the given process ID.
	Unlock(uint16)
	// Start Alerter.
	Start()
	// Stop Alerter.
	Stop()
}

// NopAlerter is an alerter that does nothing.
func NopAlerter() Alerter {
	return nopAl{}
}

type nopAl struct{}
type nopObserverManager struct{}

func newNopObserverManager() utils.ObserverManager {
	return nopObserverManager{}
}

func (nopObserverManager) RemoveObserver() {}

func (nopAl) Start()                                                      {}
func (nopAl) Stop()                                                       {}
func (nopAl) NewFork(Preunit, Preunit)                                    {}
func (nopAl) HandleIncoming(network.Connection)                           {}
func (nopAl) Disambiguate([]Unit, Preunit) (Unit, error)                  { return nil, nil }
func (nopAl) RequestCommitment(Preunit, uint16) error                     { return nil }
func (nopAl) ResolveMissingCommitment(e error, _ Preunit, _ uint16) error { return e }
func (nopAl) IsForker(uint16) bool                                        { return false }
func (nopAl) AddForkObserver(func(Preunit, Preunit)) utils.ObserverManager {
	return newNopObserverManager()
}
func (nopAl) Lock(uint16)   {}
func (nopAl) Unlock(uint16) {}
