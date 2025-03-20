package metrics

import (
	"net/http"
	"os"
	"time"

	"github.com/deckhouse/dmt/internal/flags"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

func GetInfo() prometheus.Counter {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dmt_info",
		Help: "DMT info",
	}, []string{"version", "id", "repository"}).With(prometheus.Labels{
		"id":         os.Getenv("DMT_METRICS_ID"),
		"version":    flags.Version,
		"repository": os.Getenv("DMT_REPOSITORY"), // TODO: add repository
	})
	c.Add(1)

	return c
}

func NewPusher() *push.Pusher {
	if os.Getenv("DMT_METRICS_URL") == "" {
		return nil
	}
	if os.Getenv("DMT_METRICS_TOKEN") == "" {
		return nil
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return push.New(os.Getenv("DMT_METRICS_URL"), "dmt").Client(httpClient).
		Header(http.Header{"Authorization": []string{"Bearer " + os.Getenv("DMT_METRICS_TOKEN")}})
}
