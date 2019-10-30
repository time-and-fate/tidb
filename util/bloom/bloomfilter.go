package bloom

import (
	"fmt"
	"hash/fnv"
)

// Filter a simple abstraction of bloom filter
type Filter struct {
	BitSet   []uint64
	Length   uint64
	UnitSize uint64
}

// NewFilter returns a filter with a given size
func NewFilter(length int) (*Filter, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length is not positive")
	}
	bitset := make([]uint64, length)
	bits := uint64(64)
	return &Filter{
		BitSet:   bitset,
		Length:   bits * uint64(length),
		UnitSize: bits,
	}, nil
}

// NewFilterBySlice create a bloom filter by the given slice
func NewFilterBySlice(bs []uint64) (*Filter, error) {
	if len(bs) == 0 {
		return nil, fmt.Errorf("len(bs) == 0")
	}

	bits := uint64(64)
	return &Filter{
		BitSet:   bs,
		Length:   bits * uint64(len(bs)),
		UnitSize: bits,
	}, nil
}

// Insert a key into the filter
func (bf *Filter) Insert(key []byte) {
	idx, shift := bf.hash(key)
	bf.BitSet[idx] |= 1 << shift
}

// Probe check whether the given key is in the filter
func (bf *Filter) Probe(key []byte) bool {
	idx, shift := bf.hash(key)

	return bf.BitSet[idx]&(1<<shift) != 0
}

func (bf *Filter) hash(key []byte) (uint64, uint64) {
	hash := ihash(key) % uint64(bf.Length)
	idx := hash / bf.UnitSize
	shift := hash % bf.UnitSize

	return idx, shift
}

func ihash(key []byte) uint64 {
	h := fnv.New64a()
	h.Write(key)
	return h.Sum64()
}
