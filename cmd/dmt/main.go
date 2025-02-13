package main

import (
	"os"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/pkg/config"
)

func main() {
	execute()
}

func runLint(dirs []string) {
	logger.InfoF("Dirs: %v", dirs)

	cfg := &config.RootConfig{}
	err := config.NewLoader(cfg, dirs...).Load()
	logger.CheckErr(err)

	mng := manager.NewManager(dirs, cfg)
	mng.Run()
	mng.PrintResult()

	if mng.HasCriticalErrors() {
		os.Exit(1)
	}
}
