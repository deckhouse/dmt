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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/promremote"
	"github.com/deckhouse/dmt/pkg/config"
)

func main() {
	execute()
}

func runLint(dir string) {
	color.NoColor = false
	logger.InfoF("DMT version: %s", version)
	logger.InfoF("Dir: %v", dir)

	cfg, err := config.NewDefaultRootConfig(dir)
	logger.CheckErr(err)

	mng := manager.NewManager(dir, cfg)
	mng.Run()
	mng.PrintResult()

	if err := sendMetrics(dir); err != nil {
		logger.ErrorF("Failed to send metrics: %v", err)
	}

	if mng.HasCriticalErrors() {
		os.Exit(1)
	}
}

func sendMetrics(dir string) error {
	if os.Getenv("DMT_METRICS_URL") == "" || os.Getenv("DMT_METRICS_TOKEN") == "" {
		return nil
	}

	promclient, err := promremote.NewClient(promremote.Config{
		WriteURL: os.Getenv("DMT_METRICS_URL"),
	})
	if err != nil {
		return fmt.Errorf("failed to create promremote client: %v", err)
	}

	ts := promremote.ConvertMetric(metrics.GetInfo(dir), "dmt_info")

	if _, err = promclient.WriteTimeSeries(context.Background(), []promremote.TimeSeries{ts}, promremote.WriteOptions{
		Headers: map[string]string{
			"Authorization": "Bearer " + os.Getenv("DMT_METRICS_TOKEN"),
		},
	}); err != nil {
		return fmt.Errorf("failed to send metrics: %v", err)
	}

	return nil
}
