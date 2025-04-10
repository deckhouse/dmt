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
	"os"

	"github.com/fatih/color"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
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

	// init metrics storage
	metrics.GetClient(dir)

	mng := manager.NewManager(dir, cfg)
	mng.Run()
	mng.PrintResult()

	metrics.SetDmtInfo()
	metrics.SetLinterWarningsMetrics(cfg.GlobalSettings)
	metrics.SetDmtRuntimeDuration()
	metrics.SetDmtRuntimeDurationSeconds()

	metricsClient := metrics.GetClient(dir)
	metricsClient.Send(context.Background())

	if mng.HasCriticalErrors() {
		os.Exit(1)
	}
}
