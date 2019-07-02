package gossip

type bitSet struct {
	array []byte
}

func (bs *bitSet) set(k uint32) {
	bs.array[k>>3] |= (1 << (k & 7))
}

func (bs *bitSet) test(k uint32) bool {
	return (bs.array[k>>3] & (1 << (k & 7))) != 0
}

func (bs *bitSet) toSlice() []byte {
	return bs.array
}

func newBitSet(n uint32) *bitSet {
	return &bitSet{
		array: make([]byte, (n+7)>>3),
	}
}

func bitSetFromSlice(array []byte) *bitSet {
	return &bitSet{array: array}
}
