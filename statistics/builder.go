// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package statistics

import (
	"bytes"
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
)

// SortedBuilder is used to build histograms for PK and index.
type SortedBuilder struct {
	sc              *stmtctx.StatementContext
	numBuckets      int64
	valuesPerBucket int64
	lastNumber      int64
	bucketIdx       int64
	Count           int64
	hist            *Histogram
}

// NewSortedBuilder creates a new SortedBuilder.
func NewSortedBuilder(sc *stmtctx.StatementContext, numBuckets, id int64, tp *types.FieldType) *SortedBuilder {
	return &SortedBuilder{
		sc:              sc,
		numBuckets:      numBuckets,
		valuesPerBucket: 1,
		hist:            NewHistogram(id, 0, 0, 0, tp, int(numBuckets), 0),
	}
}

// Hist returns the histogram built by SortedBuilder.
func (b *SortedBuilder) Hist() *Histogram {
	return b.hist
}

// Iterate updates the histogram incrementally.
func (b *SortedBuilder) Iterate(data types.Datum) error {
	b.Count++
	if b.Count == 1 {
		b.hist.AppendBucket(&data, &data, 1, 1)
		b.hist.NDV = 1
		return nil
	}
	cmp, err := b.hist.GetUpper(int(b.bucketIdx)).CompareDatum(b.sc, &data)
	if err != nil {
		return errors.Trace(err)
	}
	if cmp == 0 {
		// The new item has the same value as current bucket value, to ensure that
		// a same value only stored in a single bucket, we do not increase bucketIdx even if it exceeds
		// valuesPerBucket.
		b.hist.Buckets[b.bucketIdx].Count++
		b.hist.Buckets[b.bucketIdx].Repeat++
	} else if b.hist.Buckets[b.bucketIdx].Count+1-b.lastNumber <= b.valuesPerBucket {
		// The bucket still have room to store a new item, update the bucket.
		b.hist.updateLastBucket(&data, b.hist.Buckets[b.bucketIdx].Count+1, 1)
		b.hist.NDV++
	} else {
		// All buckets are full, we should merge buckets.
		if b.bucketIdx+1 == b.numBuckets {
			b.hist.mergeBuckets(int(b.bucketIdx))
			b.valuesPerBucket *= 2
			b.bucketIdx = b.bucketIdx / 2
			if b.bucketIdx == 0 {
				b.lastNumber = 0
			} else {
				b.lastNumber = b.hist.Buckets[b.bucketIdx-1].Count
			}
		}
		// We may merge buckets, so we should check it again.
		if b.hist.Buckets[b.bucketIdx].Count+1-b.lastNumber <= b.valuesPerBucket {
			b.hist.updateLastBucket(&data, b.hist.Buckets[b.bucketIdx].Count+1, 1)
		} else {
			b.lastNumber = b.hist.Buckets[b.bucketIdx].Count
			b.bucketIdx++
			b.hist.AppendBucket(&data, &data, b.lastNumber+1, 1)
		}
		b.hist.NDV++
	}
	return nil
}

// BuildColumnHist build a histogram for a column.
// numBuckets: number of buckets for the histogram.
// id: the id of the table.
// collector: the collector of samples.
// tp: the FieldType for the column.
// count: represents the row count for the column.
// ndv: represents the number of distinct values for the column.
// nullCount: represents the number of null values for the column.
func BuildColumnHist(ctx sessionctx.Context, numBuckets, id int64, collector *SampleCollector, tp *types.FieldType, count int64, ndv int64, nullCount int64) (*Histogram, error) {
	if ndv > count {
		ndv = count
	}
	if count == 0 || len(collector.Samples) == 0 {
		return NewHistogram(id, ndv, nullCount, 0, tp, 0, collector.TotalSize), nil
	}
	sc := ctx.GetSessionVars().StmtCtx
	samples := collector.Samples
	samples, err := SortSampleItems(sc, samples)
	if err != nil {
		return nil, err
	}
	hg := NewHistogram(id, ndv, nullCount, 0, tp, int(numBuckets), collector.TotalSize)

	topN := collector.TopN.TopN
	var topNTotal uint64
	for _, meta := range topN {
		topNTotal += meta.Count
	}

	topNTotalInSample := uint64(0)
	var sampleMoreThanOnce bool
	var sampleF1 int64
	var sampleNDV int64
	var lastValBytes []byte
OUTER:
	for i := 0; i < len(samples); i++ {
		valBytes, err := tablecodec.EncodeValue(ctx.GetSessionVars().StmtCtx, nil, samples[i].Value)
		if err != nil {
			return nil, err
		}
		for _, meta := range topN {
			if bytes.Compare(valBytes, meta.Encoded) == 0 {
				topNTotalInSample++
				continue OUTER
			}
		}

		if bytes.Compare(lastValBytes, valBytes) == 0 {
			sampleMoreThanOnce = true
		} else {
			sampleNDV++
			if sampleMoreThanOnce == false {
				sampleF1++
			}
			sampleMoreThanOnce = false
			lastValBytes = valBytes
		}
	}
	F1Factor := float64(ndv-int64(len(topN))-(sampleNDV-sampleF1)) / float64(sampleF1)
	sampleNum := int64(len(samples))
	if uint64(len(samples)) <= topNTotalInSample {
		// all samples are in topn
		return hg, nil
	}
	// As we use samples to build the histogram, the bucket number and repeat should multiply a factor.
	sampleFactor := float64(uint64(count)-topNTotal) / float64(uint64(len(samples))-topNTotalInSample)
	// Since bucket count is increased by sampleFactor, so the actual max values per bucket is
	// floor(valuesPerBucket/sampleFactor)*sampleFactor, which may less than valuesPerBucket,
	// thus we need to add a sampleFactor to avoid building too many buckets.
	valuesPerBucket := float64(count-int64(topNTotal))/float64(numBuckets) + sampleFactor
	if hg.NDV <= int64(len(topN)) {
		return hg, nil
	}
	ndvFactor := float64(uint64(count)-topNTotal) / float64(hg.NDV-int64(len(topN)))
	if ndvFactor > sampleFactor {
		ndvFactor = sampleFactor
	}
	bucketIdx := 0
	var lastCount int64
	var corrXYSum float64

	var i int64 = 0
	var iBktSample = i
	for {
		matched := false
		valBytes, err := tablecodec.EncodeValue(ctx.GetSessionVars().StmtCtx, nil, samples[i].Value)
		if err != nil {
			return nil, err
		}
		for _, meta := range topN {
			if bytes.Compare(valBytes, meta.Encoded) == 0 {
				i++
				matched = true
				break
			}
		}
		if matched == false {
			break
		}
	}

	hg.AppendBucket(&samples[i].Value, &samples[i].Value, int64(sampleFactor), int64(ndvFactor))
	i++
	iBktSample++

	var sampleBktNDV int64 = 1
	// f1 is NDV of items that only appear once in the samples
	var f1 int64
	// moreThanOnce helps to calc f1
	var moreThanOnce bool
	for ; i < sampleNum; i++ {
		corrXYSum += float64(i) * float64(samples[i].Ordinal)

		matched := false
		valBytes, err := tablecodec.EncodeValue(ctx.GetSessionVars().StmtCtx, nil, samples[i].Value)
		if err != nil {
			return nil, err
		}
		for _, meta := range topN {
			if bytes.Compare(valBytes, meta.Encoded) == 0 {
				matched = true
				break
			}
		}
		if matched == true {
			continue
		}

		cmp, err := hg.GetUpper(bucketIdx).CompareDatum(sc, &samples[i].Value)
		if err != nil {
			return nil, errors.Trace(err)
		}
		totalCount := float64(iBktSample+1) * sampleFactor
		if cmp == 0 {
			moreThanOnce = true
			// The new item has the same value as current bucket value, to ensure that
			// a same value only stored in a single bucket, we do not increase bucketIdx even if it exceeds
			// valuesPerBucket.
			hg.Buckets[bucketIdx].Count = int64(totalCount)
			if hg.Buckets[bucketIdx].Repeat == int64(ndvFactor) {
				hg.Buckets[bucketIdx].Repeat = int64(2 * sampleFactor)
			} else {
				hg.Buckets[bucketIdx].Repeat += int64(sampleFactor)
			}
		} else if totalCount-float64(lastCount) <= valuesPerBucket {
			if moreThanOnce == false {
				f1++
			}
			sampleBktNDV++
			moreThanOnce = false
			// The bucket still have room to store a new item, update the bucket.
			hg.updateLastBucket(&samples[i].Value, int64(totalCount), int64(ndvFactor))
		} else {
			if moreThanOnce == false {
				f1++
			}
			hg.Buckets[len(hg.Buckets)-1].NDV = int64(F1Factor*float64(f1)) + sampleBktNDV - f1
			lastCount = hg.Buckets[bucketIdx].Count
			// The bucket is full, store the item in the next bucket.
			bucketIdx++
			hg.AppendBucket(&samples[i].Value, &samples[i].Value, int64(totalCount), int64(ndvFactor))
			sampleBktNDV = 1
			f1 = 0
		}
		iBktSample++
	}
	if moreThanOnce == false {
		f1++
	}
	hg.Buckets[len(hg.Buckets)-1].NDV = int64(F1Factor*float64(f1)) + sampleBktNDV - f1
	// Compute column order correlation with handle.
	if sampleNum == 1 {
		hg.Correlation = 1
		return hg, nil
	}
	// X means the ordinal of the item in original sequence, Y means the oridnal of the item in the
	// sorted sequence, we know that X and Y value sets are both:
	// 0, 1, ..., sampleNum-1
	// we can simply compute sum(X) = sum(Y) =
	//    (sampleNum-1)*sampleNum / 2
	// and sum(X^2) = sum(Y^2) =
	//    (sampleNum-1)*sampleNum*(2*sampleNum-1) / 6
	// We use "Pearson correlation coefficient" to compute the order correlation of columns,
	// the formula is based on https://en.wikipedia.org/wiki/Pearson_correlation_coefficient.
	// Note that (itemsCount*corrX2Sum - corrXSum*corrXSum) would never be zero when sampleNum is larger than 1.
	itemsCount := float64(sampleNum)
	corrXSum := (itemsCount - 1) * itemsCount / 2.0
	corrX2Sum := (itemsCount - 1) * itemsCount * (2*itemsCount - 1) / 6.0
	hg.Correlation = (itemsCount*corrXYSum - corrXSum*corrXSum) / (itemsCount*corrX2Sum - corrXSum*corrXSum)
	return hg, nil
}

// BuildColumn builds histogram from samples for column.
func BuildColumn(ctx sessionctx.Context, numBuckets, id int64, collector *SampleCollector, tp *types.FieldType) (*Histogram, error) {
	return BuildColumnHist(ctx, numBuckets, id, collector, tp, collector.Count, collector.FMSketch.NDV(), collector.NullCount)
}
