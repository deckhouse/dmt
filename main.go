package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/logger"
)

func main() {
	var (
		printHelp    bool
		printVersion bool
		cfgFile      string
		version      = "HEAD"
	)

	logger.InitLogger()

	flagSet := pflag.NewFlagSet("d8-lint", pflag.ContinueOnError)

	flagSet.BoolVarP(&printHelp, "help", "h", false, "help message")
	flagSet.BoolVarP(&printVersion, "version", "v", false, "version message")
	flagSet.StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.d8lint.yaml)")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		Usage(flagSet)
		return
	}

	if printHelp {
		Usage(flagSet)
		return
	}

	if printVersion {
		fmt.Println("d8-lint version: ", version)
		return
	}

	var dirs = flagSet.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	cfg := config.NewDefault()
	err := config.NewLoader(cfg).Load()
	logger.CheckErr(err)
}

func Usage(flagSet *pflag.FlagSet) {
	fmt.Println("Usage: d8-lint [OPTIONS]")
	flagSet.PrintDefaults()
}
