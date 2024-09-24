package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/logger"
	"github.com/deckhouse/d8-lint/pkg/manager"
)

func main() {
	logger.InitLogger()

	dirs := parseFlags()
	logger.InfoF("Dirs: %v", dirs)

	cfg, err := config.NewDefault(dirs)
	logger.CheckErr(err)

	mng := manager.NewManager(dirs, cfg)
	result := mng.Run()
	if result.ConvertToError() != nil {
		fmt.Printf("%s\n", result.ConvertToError())
		os.Exit(1)
	}
}
