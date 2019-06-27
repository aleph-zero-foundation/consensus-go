package gossip

type bitSet struct {
	array []byte
}

func (bs *bitSet) set(k int) {
	bs.array[k>>3] |= (1 << uint32(k&7))
}

func (bs *bitSet) test(k int) bool {
	return (bs.array[k>>3] & (1 << uint32(k&7))) != 0
}

func (bs *bitSet) toSlice() []byte {
	return bs.array
}

func newBitSet(n int) *bitSet {
	return &bitSet{
		array: make([]byte, (n+7)>>3),
	}
}

func bitSetFromSlice(array []byte) *bitSet {
	return &bitSet{array: array}
}
