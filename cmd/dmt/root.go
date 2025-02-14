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
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/logger"
)

var version = "devel"

func execute() {
	rootCmd := &cobra.Command{
		Use:   "dmt",
		Short: "Dechouse module tools",
		Long:  `It's a swiss knife for everyone, who want's create, maintain or use deckhouse modules.`,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			flags.Version = version

			logger.InitLogger(flags.LogLevel)

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

	genCmd := &cobra.Command{
		Use:   "gen",
		Short: "generator for Deckhouse modules",
		Long:  `A lot of useful generators`,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("under development")
		},
		Hidden: true,
	}

	lintCmd.Flags().AddFlagSet(flags.InitLintFlagSet())

	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(genCmd)
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

	if len(dirs) == 0 {
		return
	}

	var parsedDirs []string
	for _, dir := range dirs {
		d, err := homedir.Expand(dir)
		if err != nil {
			logger.ErrorF("Error expanding directory: %v", err)
			continue
		}

		d, err = filepath.Abs(d)
		if err != nil {
			logger.ErrorF("Error expanding directory: %v\n", err)
			continue
		}

		parsedDirs = append(parsedDirs, d)
	}

	runLint(parsedDirs)
}
