package notify

import (
	"time"

	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/hostinfo"
)

type Notifier interface {
	Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error
}

func FilterSamplesByAge(samples []prompb.Sample, maxAgeSec uint) []prompb.Sample {
	treshold := time.Now().UnixMilli() - int64(maxAgeSec*1000)
	for idx, sample := range samples {
		if sample.Timestamp >= treshold {
			return samples[idx:]
		}
	}
	return []prompb.Sample{}
}
