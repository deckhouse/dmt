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

package metrics

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/promremote"
)

func GetInfo(dir string) prometheus.Counter {
	repository := cmp.Or(os.Getenv("DMT_REPOSITORY"), getRepositoryAddress(dir))
	if repository == "" {
		return nil
	}
	repositoryElements := strings.Split(repository, "/")
	repositoryID := repository
	if len(repositoryElements) > 1 {
		repositoryID = repositoryElements[len(repositoryElements)-1]
	}
	id := cmp.Or(os.Getenv("DMT_METRICS_ID"), repositoryID)

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

func Send(dir string) error {
	if os.Getenv("DMT_METRICS_URL") == "" || os.Getenv("DMT_METRICS_TOKEN") == "" {
		return nil
	}

	promclient, err := promremote.NewClient(
		promremote.NewConfig(
			promremote.WriteURLOption(os.Getenv("DMT_METRICS_URL")),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create promremote client: %w", err)
	}

	ts := promremote.ConvertMetric(GetInfo(dir), "dmt_info")

	if _, err = promclient.WriteTimeSeries(context.Background(), []promremote.TimeSeries{ts}, promremote.WriteOptions{
		Headers: map[string]string{
			"Authorization": "Bearer " + os.Getenv("DMT_METRICS_TOKEN"),
		},
	}); err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}

	return nil
}
