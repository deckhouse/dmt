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
	PrintVersion bool
	Version      string
	ValuesFile   string
	PprofFile    string
)

func InitDefaultFlagSet() *pflag.FlagSet {
	defaults := pflag.NewFlagSet("defaults for all commands", pflag.ExitOnError)

	defaults.BoolVarP(&PrintVersion, "version", "v", false, "version message")

	return defaults
}

func InitLintFlagSet() *pflag.FlagSet {
	lint := pflag.NewFlagSet("lint", pflag.ContinueOnError)

	lint.IntVarP(&LintersLimit, "parallel", "p", numThreads, "number of threads for parallel processing")

	lint.StringVar(&LinterName, "linter", "", "linter name to run")
	lint.StringVarP(&LogLevel, "log-level", "l", "INFO", "log-level [DEBUG | INFO | WARN | ERROR]")
	lint.StringVarP(&ValuesFile, "values-file", "f", "", "path to values.yaml file with override values")
	lint.StringVarP(&PprofFile, "pprof-file", "", "", "path to pprof file")

	return lint
}

func InitGenFlagSet() *pflag.FlagSet {
	gen := pflag.NewFlagSet("gen", pflag.ContinueOnError)

	return gen
}
