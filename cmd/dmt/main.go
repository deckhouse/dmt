package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/mitchellh/go-homedir"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

var version = "HEAD"

func main() {
	flags.Version = version
	color.NoColor = false

	defaults := flags.InitDefaultFlagSet()

	lint := flags.InitLintFlagSet()
	lint.AddFlagSet(defaults)

	gen := flags.InitGenFlagSet()
	gen.AddFlagSet(defaults)

	if len(os.Args) < 2 {
		flags.GeneralParse(defaults)
		defaults.Usage()
		return
	}

	switch os.Args[1] {
	case "lint":
		flags.GeneralParse(lint)

		var dirs = lint.Args()[1:]
		if len(dirs) == 0 {
			dirs = []string{"."}
		}

		if len(dirs) == 0 {
			return
		}

		var parsedDirs []string
		for _, dir := range dirs {
			d, err := homedir.Expand(dir)
			if err != nil {
				logger.ErrorF("Error expanding directory: %v", err)
				continue
			}
			d, err = filepath.Abs(d)
			if err != nil {
				logger.ErrorF("Error expanding directory: %v\n", err)
				continue
			}
			parsedDirs = append(parsedDirs, d)
		}
		runLint(parsedDirs)
	case "gen":
		flags.GeneralParse(gen)
	default:
		flags.GeneralParse(defaults)
		defaults.Usage()
	}
}

func runLint(dirs []string) {
	logger.InfoF("Dirs: %v", dirs)

	errors.Init()
	err := config.NewDefault(dirs)
	logger.CheckErr(err)

	go manager.Run(dirs)

	errs := errors.GetErrors()
	convertedError := errs.ConvertToError()
	if convertedError != nil {
		fmt.Printf("%s\n", convertedError)
	}

	if errs.Critical() {
		os.Exit(1)
	}
}
