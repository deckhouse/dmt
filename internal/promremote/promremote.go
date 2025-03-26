/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package promremote is a package to write timeseries data to a Prometheus remote write endpoint.
// copied from https://github.com/m3dbx/prometheus_remote_client_golang/blob/master/promremote/client.go
package promremote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

const (
	defaultHTTPClientTimeout = 30 * time.Second
	defaultUserAgent         = "promremote-go/1.0.0"
)

// Label is a metric label.
type Label struct {
	Name  string
	Value string
}

// TimeSeries are made of labels and a datapoint.
type TimeSeries struct {
	Labels    []Label
	Datapoint Datapoint
}

// TSList is a slice of TimeSeries.
type TSList []TimeSeries

// A Datapoint is a single data value reported at a given time.
type Datapoint struct {
	Timestamp time.Time
	Value     float64
}

// WriteOptions specifies additional write options.
type WriteOptions struct {
	// Headers to append or override the outgoing headers.
	Headers map[string]string
}

// WriteResult returns the successful HTTP status code.
type WriteResult struct {
	StatusCode int
}

// WriteError is an error that can also return the HTTP status code
// if the response is what caused an error.
type WriteError interface {
	error
	StatusCode() int
}

type Client struct {
	writeURL   string
	token      string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new remote write coordinator client.
func NewClient(url, token string) *Client {
	if url == "" || token == "" {
		return nil
	}

	httpClient := &http.Client{
		Timeout: defaultHTTPClientTimeout,
	}

	return &Client{
		token:      token,
		writeURL:   url,
		httpClient: httpClient,
		userAgent:  defaultUserAgent,
	}
}

func (c *Client) WriteTimeSeries(
	ctx context.Context,
	seriesList TSList,
	opts WriteOptions,
) (WriteResult, WriteError) {
	return c.WriteProto(ctx, seriesList.toPromWriteRequest(), opts)
}

func (c *Client) WriteProto(
	ctx context.Context,
	promWR *prompb.WriteRequest,
	opts WriteOptions,
) (WriteResult, WriteError) {
	var result WriteResult
	data, err := proto.Marshal(promWR)
	if err != nil {
		return result, writeError{err: fmt.Errorf("unable to marshal protobuf: %w", err)}
	}

	encoded := snappy.Encode(nil, data)

	body := bytes.NewReader(encoded)
	req, err := http.NewRequest("POST", c.writeURL, body)
	if err != nil {
		return result, writeError{err: err}
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set("Authorization", "Bearer "+c.token)

	if opts.Headers != nil {
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return result, writeError{err: err}
	}

	result.StatusCode = resp.StatusCode

	defer resp.Body.Close()

	if result.StatusCode/100 != 2 {
		writeErr := writeError{
			err:  fmt.Errorf("expected HTTP 200 status code: actual=%d", resp.StatusCode),
			code: result.StatusCode,
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			writeErr.err = fmt.Errorf("%w, body_read_error=%w", writeErr.err, err)
			return result, writeErr
		}

		writeErr.err = fmt.Errorf("%w, body=%s", writeErr.err, body)
		return result, writeErr
	}

	return result, nil
}

// toPromWriteRequest converts a list of timeseries to a Prometheus proto write request.
func (t TSList) toPromWriteRequest() *prompb.WriteRequest {
	promTS := make([]prompb.TimeSeries, len(t))

	for i, ts := range t {
		labels := make([]prompb.Label, len(ts.Labels))
		for j, label := range ts.Labels {
			labels[j] = prompb.Label{Name: label.Name, Value: label.Value}
		}

		sample := []prompb.Sample{{
			// Timestamp is int milliseconds for remote write.
			Timestamp: ts.Datapoint.Timestamp.UnixNano() / int64(time.Millisecond),
			Value:     ts.Datapoint.Value,
		}}
		promTS[i] = prompb.TimeSeries{Labels: labels, Samples: sample}
	}

	return &prompb.WriteRequest{
		Timeseries: promTS,
	}
}

type writeError struct {
	err  error
	code int
}

func (e writeError) Error() string {
	return e.err.Error()
}

// StatusCode returns the HTTP status code of the error if error
// was caused by the response, otherwise it will be just zero.
func (e writeError) StatusCode() int {
	return e.code
}
