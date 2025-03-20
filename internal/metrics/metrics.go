package metrics

import (
	"cmp"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/flags"
)

func GetInfo(dir string) prometheus.Counter {
	repository := cmp.Or(os.Getenv("DMT_REPOSITORY"), getRepositoryAddress(dir))
	if repository == "" {
		return nil
	}
	repositoryElements := strings.Split(repository, "/")
	if len(repositoryElements) > 1 {
		repository = repositoryElements[len(repositoryElements)-1]
		repository = strings.TrimSuffix(repository, ".git")
	}
	id := cmp.Or(os.Getenv("DMT_METRICS_ID"), repository)

	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dmt_info",
		Help: "DMT info",
	}, []string{"version", "id", "repository"}).With(prometheus.Labels{
		"id":         id,
		"version":    flags.Version,
		"repository": repository,
	})
	c.Add(1)

	return c
}
