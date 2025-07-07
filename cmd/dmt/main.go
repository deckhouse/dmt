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
	"errors"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/fatih/color"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/pkg/config"
)

func main() {
	execute()
}

func runLint(dir string) error {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorF("Critical panic recovered in runLint: %v", r)
		}
	}()

	if flags.PprofFile != "" {
		logger.InfoF("Profiling enabled. Profile file: %s", flags.PprofFile)
		defer func() {
			pproFile, err := fsutils.ExpandDir(flags.PprofFile)
			if err != nil {
				logger.ErrorF("could not get current working directory: %s", err)
				return
			}
			logger.InfoF("Writing memory profile to %s", pproFile)
			f, err := os.Create(pproFile)
			if err != nil {
				logger.ErrorF("could not create memory profile: %s", err)
				return
			}
			defer f.Close()
			runtime.GC()
			// Lookup("allocs") creates a profile similar to go test -memprofile.
			// Alternatively, use Lookup("heap") for a profile
			// that has inuse_space as the default index.
			if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
				logger.ErrorF("could not write memory profile: %s", err)
				return
			}
		}()
	}
	// enable color output for Github actions, do not remove it
	color.NoColor = false
	logger.InfoF("DMT version: %s", version)
	logger.InfoF("Dir: %v", dir)

	cfg, err := config.NewDefaultRootConfig(dir)
	logger.CheckErr(err)

	// init metrics storage, should be done before running manager
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
		return errors.New("critical errors found")
	}

	return nil
}
