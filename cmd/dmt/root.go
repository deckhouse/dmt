/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deckhouse/dmt/internal/bootstap"
	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
)

var version = "devel"

func execute() {
	rootCmd := &cobra.Command{
		Use:   "dmt",
		Short: "Deckhouse module tools",
		Long:  `It's a swiss knife for everyone, who want's create, maintain or use deckhouse modules.`,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			flags.Version = version

			logger.InitLogger(os.Stdout, flags.LogLevel)

			if flags.PrintVersion {
				fmt.Println("dmt version: ", flags.Version)
				os.Exit(0)
			}
		},
		Run: func(_ *cobra.Command, _ []string) {
		},
	}

	lintCmd := &cobra.Command{
		Use:   "lint",
		Short: "linter for Deckhouse modules",
		Long:  `A lot of useful linters to check your modules`,
		Run:   lintCmdFunc,
	}

	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "bootstrap for Deckhouse modules",
		Long:  `A lot of useful bootstraps`,
		Run:   bootstrapCmdFunc,
	}

	lintCmd.Flags().AddFlagSet(flags.InitLintFlagSet())
	bootstrapCmd.Flags().AddFlagSet(flags.InitBootstrapFlagSet())

	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.Flags().AddFlagSet(flags.InitDefaultFlagSet())

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func lintCmdFunc(_ *cobra.Command, args []string) {
	var dirs = args[0:]

	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	dir, err := fsutils.ExpandDir(dirs[0])
	if err != nil {
		logger.ErrorF("Error expanding directory: %v", err)
		return
	}

	if err := runLint(dir); err != nil {
		os.Exit(1)
	}
}

func bootstrapCmdFunc(_ *cobra.Command, args []string) {
	if len(args) == 0 {
		logger.ErrorF("Module name is required")
		os.Exit(1)
	}

	moduleName := args[0]

	if err := bootstap.RunBootstrap(
		moduleName,
		flags.BootstrapRepositoryType,
		flags.BootstrapRepositoryURL,
		flags.BootstrapDirectory,
	); err != nil {
		os.Exit(1)
	}
}
