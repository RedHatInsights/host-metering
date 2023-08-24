package notify

import (
	"fmt"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/tidwall/wal"
)

type CpuCache struct {
	path               string
	wal                *wal.Log
	lastTruncatedIndex uint64
}

func NewCpuCache(path string) (*CpuCache, error) {
	if path == "" {
		return nil, fmt.Errorf("cpuCache path cannot be empty")
	}
	w, err := wal.Open(path, nil)
	if err != nil {
		return nil, err
	}

	firstIndex, err := w.FirstIndex()
	if err != nil {
		return nil, err
	}

	return &CpuCache{
		path:               path,
		wal:                w,
		lastTruncatedIndex: firstIndex,
	}, nil
}

func (c *CpuCache) Close() error {
	return c.wal.Close()
}

func (c *CpuCache) Write(cpuCount uint) error {
	sample := prompb.Sample{
		Value:     float64(cpuCount),
		Timestamp: time.Now().UnixMilli(),
	}
	data, err := sample.Marshal()
	if err != nil {
		return err
	}

	lastIndex, err := c.wal.LastIndex()
	if err != nil {
		return err
	}

	return c.wal.Write(lastIndex+1, data)
}

func (c *CpuCache) GetAllSamples() (samples []prompb.Sample, lastIndex uint64, err error) {

	firstIndex, err := c.wal.FirstIndex()
	if err != nil {
		return nil, 0, err
	}

	// after trunctation, the last element is left, thus the first index
	// is the one after the last truncated index
	// https://github.com/tidwall/wal/issues/20
	if c.lastTruncatedIndex != 0 {
		firstIndex = c.lastTruncatedIndex + 1
	}

	lastIndex, err = c.wal.LastIndex()
	if err != nil {
		return nil, 0, err
	}

	if lastIndex == 0 || firstIndex > lastIndex {
		return samples, lastIndex, nil
	}

	for i := firstIndex; i <= lastIndex; i++ {
		data, err := c.wal.Read(i)
		if err != nil {
			return nil, 0, err
		}
		sample := prompb.Sample{}
		err = sample.Unmarshal(data)
		if err != nil {
			return nil, 0, err
		}
		samples = append(samples, sample)
	}

	return samples, lastIndex, nil
}

func (c *CpuCache) TruncateTo(index uint64) error {
	err := c.wal.TruncateFront(index)
	if err != nil {
		return err
	}
	c.lastTruncatedIndex = index
	return nil
}
