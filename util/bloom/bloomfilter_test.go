package bloom

import (
	"testing"

	"github.com/pingcap/check"
)

func TestT(t *testing.T) {
	check.TestingT(t)
}

var _ = check.Suite(&testBloomFilterSuite{})

type testBloomFilterSuite struct{}

func (s *testBloomFilterSuite) TestNewBloomFilter(c *check.C) {
	_, err1 := NewFilter(0)
	c.Assert(err1, check.NotNil)

	_, err2 := NewFilter(10)
	c.Assert(err2, check.IsNil)
}

func (s *testBloomFilterSuite) TestNewBloomFilterBySlice(c *check.C) {
	_, err1 := NewFilterBySlice(make([]uint64, 0))
	c.Assert(err1, check.NotNil)

	_, err2 := NewFilterBySlice(make([]uint64, 10))
	c.Assert(err2, check.IsNil)
}

func (s *testBloomFilterSuite) TestBasic(c *check.C) {
	bf, _ := NewFilterBySlice(make([]uint64, 10))
	bf.Insert([]byte("Heading"))
	bf.Insert([]byte("towards"))
	bf.Insert([]byte("the"))
	bf.Insert([]byte("ocean"))
	bf.Insert([]byte("blue"))

	bf.Insert([]byte("Reaching"))
	bf.Insert([]byte("for"))
	bf.Insert([]byte("the"))
	bf.Insert([]byte("stars"))

	bf.Insert([]byte("it's"))
	bf.Insert([]byte("every"))
	bf.Insert([]byte("effort"))
	bf.Insert([]byte("of"))
	bf.Insert([]byte("yours"))

	bf.Insert([]byte("Making"))
	bf.Insert([]byte("our"))
	bf.Insert([]byte("dream"))
	bf.Insert([]byte("come"))
	bf.Insert([]byte("check.IsTrue"))

	bf.Insert([]byte("Let's"))
	bf.Insert([]byte("shape"))
	bf.Insert([]byte("the"))
	bf.Insert([]byte("future"))
	bf.Insert([]byte("of"))
	bf.Insert([]byte("database"))
	bf.Insert([]byte("together"))

	c.Assert(bf.Probe([]byte("Heading")), check.IsTrue)
	c.Assert(bf.Probe([]byte("towards")), check.IsTrue)
	c.Assert(bf.Probe([]byte("the")), check.IsTrue)
	c.Assert(bf.Probe([]byte("ocean")), check.IsTrue)
	c.Assert(bf.Probe([]byte("blue")), check.IsTrue)

	c.Assert(bf.Probe([]byte("Reaching")), check.IsTrue)
	c.Assert(bf.Probe([]byte("for")), check.IsTrue)
	c.Assert(bf.Probe([]byte("the")), check.IsTrue)
	c.Assert(bf.Probe([]byte("stars")), check.IsTrue)

	c.Assert(bf.Probe([]byte("it's")), check.IsTrue)
	c.Assert(bf.Probe([]byte("every")), check.IsTrue)
	c.Assert(bf.Probe([]byte("effort")), check.IsTrue)
	c.Assert(bf.Probe([]byte("of")), check.IsTrue)
	c.Assert(bf.Probe([]byte("yours")), check.IsTrue)

	c.Assert(bf.Probe([]byte("check.IsTrue")), check.IsTrue)
	c.Assert(bf.Probe([]byte("come")), check.IsTrue)
	c.Assert(bf.Probe([]byte("dream")), check.IsTrue)
	c.Assert(bf.Probe([]byte("our")), check.IsTrue)
	c.Assert(bf.Probe([]byte("Making")), check.IsTrue)

	c.Assert(bf.Probe([]byte("together")), check.IsTrue)
	c.Assert(bf.Probe([]byte("database")), check.IsTrue)
	c.Assert(bf.Probe([]byte("of")), check.IsTrue)
	c.Assert(bf.Probe([]byte("future")), check.IsTrue)
	c.Assert(bf.Probe([]byte("the")), check.IsTrue)
	c.Assert(bf.Probe([]byte("shape")), check.IsTrue)
	c.Assert(bf.Probe([]byte("Let's")), check.IsTrue)

	c.Assert(bf.Probe([]byte("shit")), check.IsFalse)
	c.Assert(bf.Probe([]byte("fuck")), check.IsFalse)
	c.Assert(bf.Probe([]byte("foo")), check.IsFalse)
	c.Assert(bf.Probe([]byte("bar")), check.IsFalse)
}

func BenchmarkBloomInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bf, _ := NewFilterBySlice(make([]uint64, 10))
		bf.Insert([]byte("Heading"))
		bf.Insert([]byte("towards"))
		bf.Insert([]byte("the"))
		bf.Insert([]byte("ocean"))
		bf.Insert([]byte("blue"))
	}
}

func BenchmarkBloom(b *testing.B) {
	bf, _ := NewFilterBySlice(make([]uint64, 10))
	bf.Insert([]byte("Heading"))
	bf.Insert([]byte("towards"))
	bf.Insert([]byte("the"))
	bf.Insert([]byte("ocean"))
	bf.Insert([]byte("blue"))

	for i := 0; i < b.N; i++ {
		bf.Probe([]byte("Heading"))
		bf.Probe([]byte("towards"))
		bf.Probe([]byte("the"))
		bf.Probe([]byte("ocean"))
		bf.Probe([]byte("blue"))
	}
}
