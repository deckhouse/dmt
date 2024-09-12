package main

import (
	"fmt"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/logger"
	"github.com/deckhouse/d8-lint/pkg/manager"
)

func main() {
	logger.InitLogger()

	dirs := parseFlags()
	logger.InfoF("Dirs: %v", dirs)

	cfg, err := config.NewDefault()
	logger.CheckErr(err)

	logger.InfoF("Config: %#v", cfg)
	mng := manager.NewManager(dirs, cfg)
	for i := range mng.Modules {
		logger.InfoF("module[%d]: %s", i, mng.Modules[i])
	}

	result := mng.Run()
	fmt.Printf("%v\n", result)
	logger.CheckErr(result.ConvertToError())
}
