package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/flags"
	"github.com/deckhouse/d8-lint/pkg/logger"
	"github.com/deckhouse/d8-lint/pkg/manager"
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
