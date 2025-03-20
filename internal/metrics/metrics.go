package metrics

import (
	"os"

	"github.com/deckhouse/dmt/internal/flags"

	"github.com/prometheus/client_golang/prometheus"
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
