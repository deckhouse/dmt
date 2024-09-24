package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

func parseFlags() []string {
	var (
		printHelp    bool
		printVersion bool
		cfgFile      string
		version      = "HEAD"
	)

	flagSet := pflag.NewFlagSet("d8-lint", pflag.ContinueOnError)

	flagSet.BoolVarP(&printHelp, "help", "h", false, "help message")
	flagSet.BoolVarP(&printVersion, "version", "v", false, "version message")
	flagSet.StringVarP(&cfgFile, "config", "c", "", "config file (default is $(pwd)/.d8lint.yaml)")

	flagSet.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: d8-lint [OPTIONS] [dirs...]")
		flagSet.PrintDefaults()
	}

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		flagSet.Usage()
		return nil
	}

	if printHelp {
		flagSet.Usage()
		return nil
	}

	if printVersion {
		fmt.Println("d8-lint version: ", version)
		return nil
	}

	var dirs = flagSet.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	return dirs
}
