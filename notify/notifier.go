package notify

import (
	"time"

	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/hostinfo"
)

type Notifier interface {
	Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error
}

func FilterSamplesByAge(samples []prompb.Sample, maxAge time.Duration) []prompb.Sample {
	treshold := time.Now().UnixMilli() - int64(maxAge.Milliseconds())
	for idx, sample := range samples {
		if sample.Timestamp >= treshold {
			return samples[idx:]
		}
	}
	return []prompb.Sample{}
}
