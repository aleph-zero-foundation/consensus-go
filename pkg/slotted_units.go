package gomel

// A container storing slices of units.
// Usually the ids will correspond to creators of the units.
type SlottedUnits interface {
	Get(id int) []Unit
	Set(id int, us []Unit)
	Iterate(work func([]Unit) bool)
}
