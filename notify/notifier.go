package notify

import (
	"time"

	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/prometheus/prometheus/prompb"
)

type Notifier interface {
	Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error

	// HostChanged tells notifier that related information on host has changed
	HostChanged()
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
