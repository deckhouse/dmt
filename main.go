package main

import (
	"fmt"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/logger"
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
	fmt.Printf("%#v", cfg)
}
