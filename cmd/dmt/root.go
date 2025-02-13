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

func execute() {
	rootCmd := &cobra.Command{
		Use:   "dmt",
		Short: "Dechouse module tools",
		Long:  `It's a swiss knife for everyone, who want's create, maintain or use deckhouse modules.`,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
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
