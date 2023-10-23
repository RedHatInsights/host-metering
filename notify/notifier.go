package notify

import (
	"fmt"
	"time"

	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/prometheus/prometheus/prompb"
)

type NotifyError struct {
	recoverable bool
	wrappedErr  error
}

func (e *NotifyError) Error() string {
	str := "non-recoverable notify error"
	if e.recoverable {
		str = "recoverable notify error"
	}
	if e.wrappedErr == nil {
		return str
	}
	return fmt.Errorf("%s: %w", str, e.wrappedErr).Error()
}

func (e *NotifyError) Recoverable() bool {
	return e.recoverable
}

func (e *NotifyError) Unwrap() error {
	return e.wrappedErr
}

func RecoverableError(err error) *NotifyError {
	return &NotifyError{recoverable: true, wrappedErr: err}
}

func NonRecoverableError(err error) *NotifyError {
	return &NotifyError{recoverable: false, wrappedErr: err}
}

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
