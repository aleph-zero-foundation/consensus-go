package gomel

// Data is a packet of binary data that is embedded in a single unit.
type Data []byte

// DataSource is a source of units data.
type DataSource <-chan Data

// Preblock is a set of Data objects from units contained in one block (timing round).
type Preblock []Data

// PreblockSink is an output of the aleph protocol.
type PreblockSink chan<- Preblock

// ToPreblock extracts preblock from a given slice of units.
func ToPreblock(units []Unit) Preblock {
	pb := make([]Data, 0, len(units))
	for _, u := range units {
		pb = append(pb, u.Data())
	}
	return pb
}
