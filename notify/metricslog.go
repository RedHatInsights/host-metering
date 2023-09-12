package notify

import (
	"fmt"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/tidwall/wal"
)

type MetricsLog struct {
	path               string
	wal                *wal.Log
	lastTruncatedIndex uint64
}

func NewMetricsLog(path string) (*MetricsLog, error) {
	if path == "" {
		return nil, fmt.Errorf("metrics log path cannot be empty")
	}
	w, err := wal.Open(path, nil)
	if err != nil {
		return nil, err
	}

	firstIndex, err := w.FirstIndex()
	if err != nil {
		return nil, err
	}

	return &MetricsLog{
		path:               path,
		wal:                w,
		lastTruncatedIndex: firstIndex,
	}, nil
}

func (log *MetricsLog) Close() error {
	return log.wal.Close()
}

func (log *MetricsLog) Write(cpuCount uint) error {
	sample := prompb.Sample{
		Value:     float64(cpuCount),
		Timestamp: time.Now().UnixMilli(),
	}
	data, err := sample.Marshal()
	if err != nil {
		return err
	}

	lastIndex, err := log.wal.LastIndex()
	if err != nil {
		return err
	}

	return log.wal.Write(lastIndex+1, data)
}

func (log *MetricsLog) GetAllSamples() (samples []prompb.Sample, lastIndex uint64, err error) {

	firstIndex, err := log.wal.FirstIndex()
	if err != nil {
		return nil, 0, err
	}

	// after trunctation, the last element is left, thus the first index
	// is the one after the last truncated index
	// https://github.com/tidwall/wal/issues/20
	if log.lastTruncatedIndex != 0 {
		firstIndex = log.lastTruncatedIndex + 1
	}

	lastIndex, err = log.wal.LastIndex()
	if err != nil {
		return nil, 0, err
	}

	if lastIndex == 0 || firstIndex > lastIndex {
		return samples, lastIndex, nil
	}

	for i := firstIndex; i <= lastIndex; i++ {
		data, err := log.wal.Read(i)
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

func (log *MetricsLog) TruncateTo(index uint64) error {
	err := log.wal.TruncateFront(index)
	if err != nil {
		return err
	}
	log.lastTruncatedIndex = index
	return nil
}
