package main

import (
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/logger"
	"github.com/deckhouse/d8-lint/pkg/manager"
)

func main() {
	logger.InitLogger()

	dirs := parseFlags()
	if dirs == nil {
		return
	}

	cfg := config.NewDefault()
	err := config.NewLoader(cfg).Load()
	logger.CheckErr(err)

	mng := manager.NewManager(dirs)
	for i := range mng.Modules {
		logger.Infof("module[%d]: %s", i, mng.Modules[i])
	}

	result := mng.Run()
	logger.CheckErr(result.ConvertToError())
}
