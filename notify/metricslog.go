package notify

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/tidwall/wal"
)

type MetricsLog struct {
	mu   sync.Mutex
	path string
	wal  *wal.Log
}

func NewMetricsLog(path string) (*MetricsLog, error) {
	if path == "" {
		return nil, fmt.Errorf("metrics log path cannot be empty")
	}

	w, err := wal.Open(path, nil)
	if err != nil {
		return nil, err
	}

	return &MetricsLog{
		path: path,
		wal:  w,
	}, nil
}

func (log *MetricsLog) WriteSampleNow(cpuCount uint) error {
	return log.WriteSample(cpuCount, time.Now().UnixMilli())
}

func (log *MetricsLog) WriteSample(cpuCount uint, timestamp int64) error {
	log.mu.Lock()
	defer log.mu.Unlock()

	sample := &prompb.Sample{
		Value:     float64(cpuCount),
		Timestamp: timestamp,
	}

	return log.writeSample(sample)
}

func (log *MetricsLog) writeSample(sample *prompb.Sample) error {
	// Serialize the sample to get data.
	data, err := sample.Marshal()
	if err != nil {
		return err
	}

	// Write data at the end of the log.
	index, err := log.wal.LastIndex()
	if err != nil {
		return err
	}

	return log.wal.Write(index+1, data)
}

func (log *MetricsLog) GetSamples() (samples []prompb.Sample, checkpoint uint64, err error) {
	log.mu.Lock()
	defer log.mu.Unlock()

	// Mark the end of the sample series and
	// make sure that the log is not empty.
	checkpoint, err = log.getCheckpoint()
	if err != nil {
		return nil, 0, err
	}

	// Get the beginning of the sample series.
	// There is at least one entry at this point,
	// so the first index will be a valid value.
	index, err := log.wal.FirstIndex()
	if err != nil {
		return nil, 0, err
	}

	// Re-create the sample series.
	for i := index; i < checkpoint; i++ {
		// Read a sample.
		sample, err := log.readSample(i)
		if err != nil {
			return nil, 0, err
		}

		// Skip checkpoints.
		if sample == nil {
			continue
		}

		// Append samples.
		samples = append(samples, *sample)
	}

	return samples, checkpoint, nil
}

func (log *MetricsLog) getCheckpoint() (index uint64, err error) {
	// Get the latest index.
	index, err = log.wal.LastIndex()
	if err != nil {
		return 0, err
	}

	// Check the log if not empty.
	if index > 0 {

		// Get data from the latest index.
		data, err := log.wal.Read(index)
		if err != nil {
			return 0, err
		}

		// Return the checkpoint if detected.
		// A checkpoint doesn't contain any data.
		if len(data) == 0 {
			return index, nil
		}
	}

	// Otherwise, create a new checkpoint.
	return log.createCheckpoint()
}

func (log *MetricsLog) createCheckpoint() (index uint64, err error) {
	// Get the latest index.
	index, err = log.wal.LastIndex()
	if err != nil {
		return 0, err
	}

	// Create a new checkpoint.
	if log.wal.Write(index+1, nil) != nil {
		return 0, err
	}

	return index + 1, nil
}

func (log *MetricsLog) readSample(index uint64) (*prompb.Sample, error) {
	// Get data from the specified index.
	data, err := log.wal.Read(index)
	if err != nil {
		return nil, err
	}

	// Ignore checkpoints.
	if len(data) == 0 {
		return nil, nil
	}

	// Deserialize the data to get a sample.
	sample := &prompb.Sample{}
	err = sample.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	return sample, nil
}

func (log *MetricsLog) RemoveSamples(checkpoint uint64) error {
	log.mu.Lock()
	defer log.mu.Unlock()

	// Remove all data entries that are before the specified checkpoint.
	return log.wal.TruncateFront(checkpoint)
}

func (log *MetricsLog) RemoveOldestSamples(numSamples int) error {
	if numSamples <= 0 {
		return nil
	}

	log.mu.Lock()
	defer log.mu.Unlock()

	firstIndex, err := log.wal.FirstIndex()
	if err != nil {
		return err
	}

	lastIndex, err := log.wal.LastIndex()
	if err != nil {
		return err
	}

	// Find index after the last sample to be removed.
	samplesRead := 0
	truncateIndex := firstIndex
	for i := firstIndex; i <= lastIndex; i++ {
		sample, err := log.readSample(i)
		if err != nil {
			return err
		}

		// Skip checkpoints.
		if sample == nil {
			continue
		}

		samplesRead++
		truncateIndex = i + 1

		if samplesRead >= numSamples {
			break
		}
	}
	if truncateIndex > lastIndex {
		truncateIndex = lastIndex
	}

	return log.wal.TruncateFront(truncateIndex)
}

func (log *MetricsLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()

	return log.wal.Close()
}
