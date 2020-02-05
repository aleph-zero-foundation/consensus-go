package gomel

// Adder is a component that accepts incoming preunits.
type Adder interface {
	// AddPrenits adds preunits received from the given process.
	AddPreunits(uint16, ...Preunit) []error
	// Close stops the Adder.
	Close()
}
