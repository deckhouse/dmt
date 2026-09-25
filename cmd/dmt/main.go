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
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/fatih/color"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/version"
	"github.com/deckhouse/dmt/pkg/config"
)

func main() {
	execute()
}

// runLint is the whole of a lint run: everything but where the modules come from,
// which is the source's business. Both `dmt lint <dir>` and `dmt lint remote <ref>`
// enter here, so setup and teardown exist once.
func runLint(ctx context.Context, src manager.Source) error {
	if flags.PprofFile != "" {
		log.Info("Profiling enabled", slog.String("file", flags.PprofFile))

		defer func() {
			pproFile, err := fsutils.ExpandDir(flags.PprofFile)
			if err != nil {
				log.Error("could not get current working directory", log.Err(err))
				return
			}

			log.Info("Writing memory profile", slog.String("file", pproFile))

			f, err := os.Create(pproFile)
			if err != nil {
				log.Error("could not create memory profile", log.Err(err))
				return
			}
			defer f.Close()

			runtime.GC()
			// Lookup("allocs") creates a profile similar to go test -memprofile.
			// Alternatively, use Lookup("heap") for a profile
			// that has inuse_space as the default index.
			if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
				log.Error("could not write memory profile", log.Err(err))
				return
			}
		}()
	}
	// enable color output for Github actions, do not remove it
	color.NoColor = false

	log.Info("DMT version", slog.String("version", version.Version), slog.String("commit", version.Commit), slog.String("date", version.Date))

	dir := src.ConfigDir()

	cfg, err := config.NewDefaultRootConfig(dir)
	if err != nil {
		return fmt.Errorf("default root config: %w", err)
	}

	// init metrics storage, should be done before running manager
	metrics.GetClient(dir)

	mng := manager.New(cfg, src)
	defer mng.Close()

	sourceErr := mng.Run(ctx)

	if flags.Fix {
		mng.ApplyFixes()
	}

	mng.PrintResult()
	mng.PrintStatistics()

	metrics.Flush(ctx, mng.MetricsSections()...)

	// A source failure — a registry that would not answer — is reported after the
	// findings, never instead of them: whatever was linted still has to be seen.
	if sourceErr != nil {
		return sourceErr
	}

	if mng.HasCriticalErrors() {
		return errors.New("critical errors found")
	}

	return nil
}
