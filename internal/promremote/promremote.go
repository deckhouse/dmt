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

// Package promremote writes timeseries to a Prometheus remote-write endpoint.
//
// It is a thin wrapper around the upstream Prometheus remote-write client
// (github.com/prometheus/prometheus/storage/remote.NewWriteClient) that
// preserves the historical TSList/TimeSeries surface used elsewhere in dmt.
package promremote

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

const (
	defaultHTTPClientTimeout = 30 * time.Second
	defaultClientName        = "promremote-go/1.0.0"
)

// Label is a metric label.
type Label struct {
	Name  string
	Value string
}

// TimeSeries is a labelled datapoint.
type TimeSeries struct {
	Labels    []Label
	Datapoint Datapoint
}

// TSList is a slice of TimeSeries.
type TSList []TimeSeries

// Datapoint is a single value reported at a given time.
type Datapoint struct {
	Timestamp time.Time
	Value     float64
}

// WriteOptions specifies additional write options. Reserved for future use;
// the upstream client manages its headers (Content-Type/Encoding, User-Agent,
// X-Prometheus-Remote-Write-Version, Authorization) internally.
type WriteOptions struct {
	Headers map[string]string
}

// WriteResult returns the successful HTTP status code.
type WriteResult struct {
	StatusCode int
}

// WriteError is an error that can also report an HTTP status code when the
// failure was caused by the response.
type WriteError interface {
	error
	StatusCode() int
}

// Client wraps an upstream Prometheus remote.WriteClient.
type Client struct {
	inner remote.WriteClient
}

// NewClient creates a remote-write client backed by the default upstream
// Prometheus client. Returns nil when url or token is empty, matching the
// previous behaviour relied on by callers.
func NewClient(rawURL, token string) *Client {
	if rawURL == "" || token == "" {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	inner, err := remote.NewWriteClient(defaultClientName, &remote.ClientConfig{
		URL:     &config_util.URL{URL: parsed},
		Timeout: model.Duration(defaultHTTPClientTimeout),
		HTTPClientConfig: config_util.HTTPClientConfig{
			BearerToken: config_util.Secret(token),
		},
	})
	if err != nil {
		return nil
	}

	return &Client{inner: inner}
}

// WriteTimeSeries serialises seriesList to the Prometheus remote-write v1
// protobuf, snappy-encodes it, and POSTs it to the configured endpoint.
func (c *Client) WriteTimeSeries(
	ctx context.Context,
	seriesList TSList,
	_ WriteOptions,
) (WriteResult, WriteError) {
	return c.WriteProto(ctx, seriesList.toPromWriteRequest())
}

// WriteProto sends an already-built prompb.WriteRequest to the configured
// endpoint.
func (c *Client) WriteProto(
	ctx context.Context,
	promWR *prompb.WriteRequest,
) (WriteResult, WriteError) {
	if c == nil || c.inner == nil {
		return WriteResult{}, &writeError{err: fmt.Errorf("promremote client is not configured")}
	}

	data, err := proto.Marshal(promWR)
	if err != nil {
		return WriteResult{}, &writeError{err: fmt.Errorf("marshal write request: %w", err)}
	}

	encoded := snappy.Encode(nil, data)

	if _, err = c.inner.Store(ctx, encoded, 0); err != nil {
		return WriteResult{}, &writeError{err: err}
	}

	return WriteResult{StatusCode: http.StatusOK}, nil
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

func (e *writeError) Error() string {
	return e.err.Error()
}

// StatusCode returns the HTTP status code of the error if the error was caused
// by the response, otherwise zero.
func (e *writeError) StatusCode() int {
	return e.code
}
