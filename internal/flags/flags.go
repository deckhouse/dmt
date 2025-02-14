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

package flags

import (
	"fmt"
	"os"

	"github.com/deckhouse/dmt/internal/logger"

	"github.com/spf13/pflag"
)

const (
	numThreads = 10
)

var (
	LintersLimit int
	LogLevel     string
	LinterName   string
)

var (
	PrintHelp    bool
	PrintVersion bool
	Version      string
)

func InitDefaultFlagSet() *pflag.FlagSet {
	defaults := pflag.NewFlagSet("defaults for all commands", pflag.ExitOnError)
	defaults.BoolVarP(&PrintHelp, "help", "h", false, "help message")
	defaults.BoolVarP(&PrintVersion, "version", "v", false, "version message")

	defaults.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt [gen|lint] [OPTIONS]")
		defaults.PrintDefaults()
	}

	return defaults
}

func InitLintFlagSet() *pflag.FlagSet {
	lint := pflag.NewFlagSet("lint", pflag.ContinueOnError)

	lint.IntVarP(&LintersLimit, "parallel", "p", numThreads, "number of threads for parallel processing")
	lint.StringVarP(&LogLevel, "log-level", "l", "INFO", "log-level [DEBUG | INFO | WARN | ERROR]")
	lint.StringVar(&LinterName, "linter", "", "linter name to run")

	lint.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt lint [OPTIONS] [dirs...]")
		lint.PrintDefaults()
	}

	return lint
}

func InitGenFlagSet() *pflag.FlagSet {
	gen := pflag.NewFlagSet("gen", pflag.ContinueOnError)

	gen.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt gen [OPTIONS]")
		pflag.PrintDefaults()
	}

	return gen
}

func GeneralParse(flagSet *pflag.FlagSet) {
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		flagSet.Usage()
		os.Exit(0)
	}

	logger.InitLogger(LogLevel)

	if PrintHelp {
		flagSet.Usage()
		os.Exit(0)
	}

	if PrintVersion {
		fmt.Println("dmt version: ", Version)
		os.Exit(0)
	}
}
