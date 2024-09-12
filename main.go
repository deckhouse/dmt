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

	cfg := config.NewDefault()
	err := config.NewLoader(cfg).Load()
	logger.CheckErr(err)

	mng := manager.NewManager(dirs)
	// for i := range mng.Modules {
	// 	logger.InfoF("module[%d]: %s", i, mng.Modules[i])
	// }

	result := mng.Run()
	fmt.Printf("%v\n", result.ConvertToError())
	logger.CheckErr(result.ConvertToError())
}
