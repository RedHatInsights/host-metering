package notify

import (
	"strings"
	"testing"

	"github.com/RedHatInsights/host-metering/hostinfo"
	"github.com/prometheus/prometheus/prompb"
)

type ShouldNotifyTestCase struct {
	name     string
	samples  []prompb.Sample
	hostInfo *hostinfo.HostInfo
	expected string
}

func TestGeneralNotifyPolicy(t *testing.T) {

	p := &GeneralNotifyPolicy{}
	correctSamples := []prompb.Sample{{}}
	correctHostInfo := fullyDefinedMockHostInfo()

	emptyHostIdHI := fullyDefinedMockHostInfo()
	emptyHostIdHI.HostId = ""

	emptyExternalOrganizationHI := fullyDefinedMockHostInfo()
	emptyExternalOrganizationHI.ExternalOrganization = ""

	testCases := []ShouldNotifyTestCase{
		{
			name:     "No samples",
			samples:  []prompb.Sample{},
			hostInfo: correctHostInfo,
			expected: "no samples to send",
		},
		{
			name:     "Nil hostInfo",
			samples:  correctSamples,
			hostInfo: nil,
			expected: "missing internal HostInfo",
		},
		{
			name:     "Empty HostId",
			samples:  correctSamples,
			hostInfo: emptyHostIdHI,
			expected: "missing HostId",
		},
		{
			name:     "Empty ExternalOrganization",
			samples:  correctSamples,
			hostInfo: emptyExternalOrganizationHI,
			expected: "missing ExternalOrganization",
		},
	}

	t.Run("Test ShouldNotify", func(t *testing.T) {

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// when
				err := p.ShouldNotify(tc.samples, tc.hostInfo)

				// then
				expectErrorContains(t, err, tc.expected)
			})
		}

		t.Run("Possitive", func(t *testing.T) {
			// when
			err := p.ShouldNotify(correctSamples, correctHostInfo)

			// then
			if err != nil {
				t.Fatalf("expected no error, got '%s'", err.Error())
			}
		})
	})
}

func fullyDefinedMockHostInfo() *hostinfo.HostInfo {
	return &hostinfo.HostInfo{
		HostId:               "hostid",
		ExternalOrganization: "externalorganization",
		SocketCount:          "socketcount",
		Product:              "product",
		Support:              "support",
		Usage:                "usage",
		Billing:              fullyDefinedMockBillingInfo(),
	}
}

func fullyDefinedMockBillingInfo() hostinfo.BillingInfo {
	return hostinfo.BillingInfo{
		Model:                 "model",
		Marketplace:           "marketplace",
		MarketplaceAccount:    "marketplaceaccount",
		MarketplaceInstanceId: "marketplaceinstanceid",
	}
}

func expectErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error to contain '%s', got '%s'", expected, err.Error())
	}
}
