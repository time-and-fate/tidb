package bloom

import (
	"fmt"
	"hash/fnv"
)

// BloomFilter a simple abstraction of bloom filter
type BloomFilter struct {
	bitSet   []uint64
	length   uint64
	unitSize uint64
}

func NewBloomFilter(length int) (*BloomFilter, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length is not positive")
	}
	bitset := make([]uint64, length)
	bits := uint64(64)
	return &BloomFilter{
		bitSet:   bitset,
		length:   bits * uint64(length),
		unitSize: bits,
	}, nil
}

// NewBloomFilterBySlice create a bloom filter by the given slice
func NewBloomFilterBySlice(bs []uint64) (*BloomFilter, error) {
	if len(bs) == 0 {
		return nil, fmt.Errorf("len(bs) == 0")
	}

	bits := uint64(64)
	return &BloomFilter{
		bitSet:   bs,
		length:   bits * uint64(len(bs)),
		unitSize: bits,
	}, nil
}

// Insert a key into the filter
func (bf *BloomFilter) Insert(key []byte) {
	idx, shift := bf.hash(key)
	bf.bitSet[idx] |= 1 << shift
}

// Probe check whether the given key is in the filter
func (bf *BloomFilter) Probe(key []byte) bool {
	idx, shift := bf.hash(key)

	return bf.bitSet[idx]&(1<<shift) != 0
}

func (bf *BloomFilter) hash(key []byte) (uint64, uint64) {
	hash := ihash(key) % uint64(bf.length)
	idx := hash / bf.unitSize
	shift := hash % bf.unitSize

	return idx, shift
}

func ihash(key []byte) uint64 {
	h := fnv.New64a()
	h.Write(key)
	return h.Sum64()
}
