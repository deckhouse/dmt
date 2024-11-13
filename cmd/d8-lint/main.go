package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/d8-lint/internal/flags"
	"github.com/deckhouse/d8-lint/internal/logger"
	"github.com/deckhouse/d8-lint/internal/manager"
	"github.com/deckhouse/d8-lint/pkg/config"
)

func main() {
	dirs := flags.ParseFlags()
	if len(dirs) == 0 {
		return
	}

	logger.InitLogger()
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
