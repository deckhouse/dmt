package flags

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

const (
	numThreads = 10
)

var (
	LintersLimit int
	LogLevel     string
)

var (
	PrintHelp    bool
	PrintVersion bool
	Version      = "HEAD"
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
	if err := flagSet.Parse(os.Args[2:]); err != nil {
		flagSet.Usage()
		os.Exit(0)
	}

	if PrintHelp {
		flagSet.Usage()
		os.Exit(0)
	}

	if PrintVersion {
		fmt.Println("dmt version: ", Version)
		os.Exit(0)
	}
}
