package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/pkg/config"
)

var Version = "HEAD"

func main() {
	flags.Version = Version
	logger.InitLogger()

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

		var dirs = lint.Args()
		if len(dirs) == 0 {
			dirs = []string{"."}
		}

		if len(dirs) == 0 {
			return
		}

		runLint(dirs)
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

	mng := manager.NewManager(dirs, cfg)
	result := mng.Run()
	if result.ConvertToError() != nil {
		fmt.Printf("%s\n", result.ConvertToError())
	}

	if result.Critical() {
		os.Exit(1)
	}
}
