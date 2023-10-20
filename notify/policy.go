package notify

import (
	"fmt"

	"github.com/prometheus/prometheus/prompb"

	"github.com/RedHatInsights/host-metering/hostinfo"
)

type NotifyPolicy interface {
	ShouldNotify([]prompb.Sample, *hostinfo.HostInfo) error
}

type GeneralNotifyPolicy struct{}

func (p *GeneralNotifyPolicy) ShouldNotify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error {

	count := len(samples)
	if count == 0 {
		return fmt.Errorf("no samples to send")
	}

	if hostinfo == nil {
		return fmt.Errorf("missing internal HostInfo")
	}

	if hostinfo.HostId == "" {
		return fmt.Errorf("missing HostId")
	}

	if hostinfo.ExternalOrganization == "" {
		return fmt.Errorf("missing ExternalOrganization")
	}

	return nil
}
