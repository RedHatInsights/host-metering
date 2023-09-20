package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
	"redhat.com/milton/logger"
)

// See https://prometheus.io/docs/concepts/remote_write_spec/ for specifiction of Prometheus remote write

const requestTimeout time.Duration = 60 * time.Second

// Should be used only for testing
var tlsInsecureSkipVerify = false

type PrometheusNotifier struct {
	cfg    *config.Config
	lastId string
	client *http.Client
}

func NewPrometheusNotifier(cfg *config.Config) *PrometheusNotifier {
	return &PrometheusNotifier{
		cfg: cfg,
	}
}

func (n *PrometheusNotifier) Notify(samples []prompb.Sample, hostinfo *hostinfo.HostInfo) error {
	if n.lastId != hostinfo.HostId {
		if err := n.createHttpClient(); err != nil {
			return err
		}
		n.lastId = hostinfo.HostId
	}
	request, err := newPrometheusRequest(hostinfo, n.cfg, samples)
	if err != nil {
		return err
	}
	return prometheusRemoteWrite(n.client, n.cfg, request)
}

func (n *PrometheusNotifier) createHttpClient() error {
	keypair, err := tls.LoadX509KeyPair(n.cfg.HostCertPath, n.cfg.HostCertKeyPath)
	if err != nil {
		return err
	}

	n.client, err = newMTLSHttpClient(keypair)
	return err
}

func newMTLSHttpClient(keypair tls.Certificate) (*http.Client, error) {
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{keypair},
		InsecureSkipVerify: tlsInsecureSkipVerify,
	}
	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

func prometheusRemoteWrite(httpClient *http.Client, cfg *config.Config, httpRequest *http.Request) error {
	var attempt uint = 0
	var maxRetryWait = time.Duration(cfg.WriteRetryMaxIntSec) * time.Second
	retryWait := time.Duration(cfg.WriteRetryMinIntSec) * time.Second

	for attempt < cfg.WriteRetryAttempts {
		resp, err := httpClient.Do(httpRequest)

		if err != nil {
			return fmt.Errorf("PrometheusRemoteWrite: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return nil // success
		}
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			logger.Debugf("PrometheusRemoteWrite: Response body: %s\n", string(body))
		}

		if resp.StatusCode/100 == 5 || resp.StatusCode == 429 {
			attempt++
			retryWait = retryWait * 2
			if retryWait > maxRetryWait {
				retryWait = maxRetryWait
			}

			logger.Infof("PrometheusRemoteWrite: Http Error: %d, retrying\n", resp.StatusCode)
			time.Sleep(retryWait)
			continue
		}
		if resp.StatusCode/100 == 4 {
			return fmt.Errorf("PrometheusRemoteWrite: Http Error: %d, failing", resp.StatusCode)
		}
		return fmt.Errorf("PrometheusRemoteWrite: Unexpected Http Status: %d", resp.StatusCode)
	}

	return nil
}

func newPrometheusRequest(hostinfo *hostinfo.HostInfo, cfg *config.Config, samples []prompb.Sample) (
	*http.Request, error) {
	writeRequest := hostInfo2WriteRequest(hostinfo, samples)
	compressedData, err := writeRequest2Payload(writeRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", cfg.WriteUrl, bytes.NewReader(compressedData))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("User-Agent", "milton/0.1.0")

	return req, nil
}

func filterEmptyLabels(labels []prompb.Label) []prompb.Label {
	var result []prompb.Label
	for _, label := range labels {
		if label.Value != "" {
			result = append(result, label)
		}
	}
	return result
}

func hostInfo2WriteRequest(hostinfo *hostinfo.HostInfo, samples []prompb.Sample) *prompb.WriteRequest {
	// Labels must be sorted by name
	labels := []prompb.Label{
		{
			Name:  "__name__",
			Value: "system_cpu_logical_count",
		},
		{
			Name:  "_id",
			Value: hostinfo.HostId,
		},
		{
			Name:  "billing_marketplace",
			Value: hostinfo.Billing.Marketplace,
		},
		{
			Name:  "billing_marketplace_account",
			Value: hostinfo.Billing.MarketplaceAccount,
		},
		{
			Name:  "billing_marketplace_instance_id",
			Value: hostinfo.Billing.MarketplaceInstanceId,
		},
		{
			Name:  "billing_model",
			Value: hostinfo.Billing.Model,
		},
		{
			Name:  "product",
			Value: hostinfo.Product,
		},
		{
			Name:  "socket_count",
			Value: hostinfo.SocketCount,
		},
		{
			Name:  "support",
			Value: hostinfo.Support,
		},
		{
			Name:  "usage",
			Value: hostinfo.Usage,
		},
	}

	labels = filterEmptyLabels(labels)

	writeRequest := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels:  labels,
				Samples: samples,
			},
		},
	}

	return writeRequest
}

func writeRequest2Payload(writeRequest *prompb.WriteRequest) ([]byte, error) {
	data, err := proto.Marshal(writeRequest)
	if err != nil {
		return data, err
	}
	compressedData := snappy.Encode(nil, data)
	return compressedData, nil
}
