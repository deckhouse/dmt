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

	cfg, err := config.NewDefault(dirs)
	logger.CheckErr(err)

	config.GlobalExcludes = cfg.LintersSettings.DeepCopy()

	mng := manager.NewManager(dirs, cfg)
	result := mng.Run()
	convertedError := result.ConvertToError()
	if convertedError != nil {
		fmt.Printf("%s\n", convertedError)
	}

	if len(config.GlobalExcludes.Container.SkipContainers) > 0 {
		fmt.Println("== Unused excludes in containers lint")
		for _, line := range config.GlobalExcludes.Container.SkipContainers {
			fmt.Printf("  * %s\n", line)
		}
	}

	if result.Critical() {
		os.Exit(1)
	}
}
