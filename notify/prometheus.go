package notify

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"redhat.com/milton/config"
	"redhat.com/milton/hostinfo"
)

// alternative to prombp could be:
// go get buf.build/gen/go/prometheus/prometheus/protocolbuffers/go@latest

// See https://prometheus.io/docs/concepts/remote_write_spec/
// TODO:
// - Persistence It is recommended that Prometheus Remote Write compatible senders should persistently buffer sample data in the event of outages in the receiver.
//

const retryWait = 1 * time.Second

func PrometheusRemoteWrite(hostinfo *hostinfo.HostInfo, cfg *config.Config) error {
	req, err := NewPrometheusRequest(hostinfo, cfg)
	if err != nil {
		return err
	}

	attempt := 0
	retries := 3

	for attempt < retries {
		client := NewMTLSHttpClient(hostinfo)
		resp, err := client.Do(req)

		if err != nil {
			return fmt.Errorf("PrometheusRemoteWrite: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return nil // success
		}
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			fmt.Println("PrometheusRemoteWrite: Response body:", string(body))
		}

		if resp.StatusCode/100 == 5 || resp.StatusCode == 429 {
			attempt++
			fmt.Printf("PrometheusRemoteWrite: Http Error: %d, retrying\n", resp.StatusCode)
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

func NewPrometheusRequest(hostinfo *hostinfo.HostInfo, cfg *config.Config) (*http.Request, error) {
	writeRequest := hostInfo2WriteRequest(hostinfo)
	compressedData, err := writeRequest2Payload(writeRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", cfg.PrometheusUrl, bytes.NewReader(compressedData))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("User-Agent", "milton/0.1.0")

	return req, nil
}

func hostInfo2WriteRequest(hostinfo *hostinfo.HostInfo) *prompb.WriteRequest {

	now := time.Now()
	timestamp := now.UnixMilli()

	labels := []prompb.Label{
		{
			Name:  "__name__",
			Value: "system_cpu_logical_count",
		},
		{
			Name:  "hostId",
			Value: hostinfo.HostId,
		},
	}

	samples := []prompb.Sample{
		{
			Value:     float64(hostinfo.CpuCount),
			Timestamp: timestamp,
		},
	}

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
