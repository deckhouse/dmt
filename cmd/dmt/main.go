package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/pkg/config"
)

const (
	numThreads = 10
)

func main() {
	var (
		printHelp    bool
		printVersion bool
		version      = "HEAD"
	)

	logger.InitLogger()

	defaults := pflag.NewFlagSet("defaults for all commands", pflag.ExitOnError)
	defaults.BoolVarP(&printHelp, "help", "h", false, "help message")
	defaults.BoolVarP(&printVersion, "version", "v", false, "version message")

	defaults.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt [gen|lint] [OPTIONS]")
		defaults.PrintDefaults()
	}

	lint := pflag.NewFlagSet("lint", pflag.ContinueOnError)
	lint.AddFlagSet(defaults)

	lint.IntVarP(&flags.LintersLimit, "parallel", "p", numThreads, "number of threads for parallel processing")
	lint.StringVarP(&flags.LogLevel, "log-level", "l", "INFO", "log-level [DEBUG | INFO | WARN | ERROR]")

	lint.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt lint [OPTIONS] [dirs...]")
		lint.PrintDefaults()
	}

	gen := pflag.NewFlagSet("gen", pflag.ContinueOnError)
	gen.AddFlagSet(defaults)

	gen.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt gen [OPTIONS]")
		defaults.PrintDefaults()
	}

	if len(os.Args) < 2 {
		defaults.Usage()
		return
	}

	switch os.Args[1] {
	case "lint":
		if err := lint.Parse(os.Args[2:]); err != nil {
			lint.Usage()
			return
		}

		if printHelp {
			lint.Usage()
			return
		}

		if printVersion {
			fmt.Println("dmt version: ", version)
			return
		}

		var dirs = lint.Args()
		if len(dirs) == 0 {
			dirs = []string{"."}
		}

		if len(dirs) == 0 {
			return
		}

		logger.InfoF("Dirs: %v", dirs)

		cfg, err := config.NewDefault(dirs)
		logger.CheckErr(err)

		mng := manager.NewManager(dirs, cfg)
		result := mng.Run()
		if result.ConvertToError() != nil {
			fmt.Printf("%s\n", result.ConvertToError())
		}

		if result.Critical() {
			os.Exit(1)
		}
	case "gen":
		if err := gen.Parse(os.Args[2:]); err != nil {
			gen.Usage()
			return
		}

		if printVersion {
			fmt.Println("dmt version: ", version)
			return
		}

		if printHelp {
			gen.Usage()
			return
		}
	default:
		if err := defaults.Parse(os.Args[1:]); err != nil {
			defaults.Usage()
			return
		}

		if printVersion {
			fmt.Println("dmt version: ", version)
			return
		}

		defaults.Usage()
		return
	}
}
