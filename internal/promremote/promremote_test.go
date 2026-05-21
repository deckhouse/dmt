/*
Copyright 2026 Flant JSC

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

package promremote

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ReturnsNilWhenURLOrTokenMissing(t *testing.T) {
	assert.Nil(t, NewClient("", "token"))
	assert.Nil(t, NewClient("http://example.com", ""))
	assert.Nil(t, NewClient("", ""))
}

func TestClient_WriteTimeSeries_PostsSnappyEncodedProtoWithBearerToken(t *testing.T) {
	const token = "secret-token"

	var (
		gotAuth        atomic.Value
		gotContentType atomic.Value
		gotEncoding    atomic.Value
		gotProtoVer    atomic.Value
		gotSeriesCount atomic.Int32
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotContentType.Store(r.Header.Get("Content-Type"))
		gotEncoding.Store(r.Header.Get("Content-Encoding"))
		gotProtoVer.Store(r.Header.Get("X-Prometheus-Remote-Write-Version"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		decoded, err := snappy.Decode(nil, body)
		require.NoError(t, err)

		var wr prompb.WriteRequest
		require.NoError(t, proto.Unmarshal(decoded, &wr))

		gotSeriesCount.Store(int32(len(wr.Timeseries)))

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, token)
	require.NotNil(t, c)

	res, werr := c.WriteTimeSeries(context.Background(), TSList{
		{
			Labels: []Label{{Name: "__name__", Value: "dmt_test"}},
			Datapoint: Datapoint{
				Timestamp: time.Unix(1700000000, 0),
				Value:     42,
			},
		},
	}, WriteOptions{})

	assert.Nil(t, werr)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "Bearer "+token, gotAuth.Load())
	assert.Equal(t, "application/x-protobuf", gotContentType.Load())
	assert.Equal(t, "snappy", gotEncoding.Load())
	assert.Equal(t, "0.1.0", gotProtoVer.Load())
	assert.Equal(t, int32(1), gotSeriesCount.Load())
}

func TestClient_WriteTimeSeries_ServerErrorReturnsWriteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	require.NotNil(t, c)

	_, werr := c.WriteTimeSeries(context.Background(), TSList{}, WriteOptions{})
	require.NotNil(t, werr)
	assert.Contains(t, werr.Error(), "400")
}
