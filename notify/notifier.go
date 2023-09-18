package notify

import (
	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/hostinfo"
)

type Notifier interface {
	Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error
}
