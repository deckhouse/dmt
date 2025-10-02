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
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/deckhouse/dmt/internal/bootstrap"
	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/logger"
)

var version = "devel"

var kebabCaseRegex = regexp.MustCompile(`^([a-z][a-z0-9]*)(-[a-z0-9]+)*$`)

func execute() {
	rootCmd := &cobra.Command{
		Use:   "dmt",
		Short: "Deckhouse module tools",
		Long:  `It's a swiss knife for everyone, who want's create, maintain or use deckhouse modules.`,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		Version: version,
	}

	rootCmd.SetVersionTemplate("dmt version: {{.Version}}\n")

	lintCmd := &cobra.Command{
		Use:   "lint",
		Short: "linter for Deckhouse modules",
		Long:  `A lot of useful linters to check your modules`,
		Run:   lintCmdFunc,
	}

	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap [module-name]",
		Short: "bootstrap for Deckhouse modules",
		Long:  `Bootstrap functionality for module development process`,
		Args:  cobra.ExactArgs(1),
		// in persistent pre run we must check all args and flags
		PersistentPreRunE: func(_ *cobra.Command, args []string) error {
			// check module name in kebab case
			moduleName := args[0]
			if !kebabCaseRegex.MatchString(moduleName) {
				return errors.New("module name must be in kebab case")
			}

			// Check flags.BootstrapRepositoryType
			repositoryType := strings.ToLower(flags.BootstrapRepositoryType)
			if repositoryType != "github" && repositoryType != "gitlab" {
				return fmt.Errorf("invalid repository type: %s", repositoryType)
			}

			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			moduleName := args[0]
			repositoryType := strings.ToLower(flags.BootstrapRepositoryType)

			config := bootstrap.BootstrapConfig{
				ModuleName:     moduleName,
				RepositoryType: repositoryType,
				RepositoryURL:  flags.BootstrapRepositoryURL,
				Directory:      flags.BootstrapDirectory,
			}

			if err := bootstrap.RunBootstrap(config); err != nil {
				return fmt.Errorf("running bootstrap: %w", err)
			}

			w := new(tabwriter.Writer)

			const minWidth = 5

			buf := bytes.NewBuffer([]byte{})
			w.Init(buf, minWidth, 0, 0, ' ', 0)

			switch repositoryType {
			case bootstrap.RepositoryTypeGitHub:
				fmt.Fprintln(w)
				color.New(color.FgHiYellow).Fprintln(w, "Don't forget to add secrets to your GitHub repository:")
				fmt.Fprintf(w, "\t%s\n", "- DECKHOUSE_PRIVATE_REPO")
				fmt.Fprintf(w, "\t%s\n", "- DEFECTDOJO_API_TOKEN")
				fmt.Fprintf(w, "\t%s\n", "- DEFECTDOJO_HOST")
				fmt.Fprintf(w, "\t%s\n", "- DEV_MODULES_REGISTRY_PASSWORD")
				fmt.Fprintf(w, "\t%s\n", "- GOPROXY")
				fmt.Fprintf(w, "\t%s\n", "- PROD_MODULES_READ_REGISTRY_PASSWORD")
				fmt.Fprintf(w, "\t%s\n", "- PROD_MODULES_REGISTRY_PASSWORD")
				fmt.Fprintf(w, "\t%s\n", "- SOURCE_REPO")
				fmt.Fprintf(w, "\t%s\n", "- SOURCE_REPO_SSH_KEY")
			case bootstrap.RepositoryTypeGitLab:
				fmt.Fprintln(w)
				color.New(color.FgHiYellow).Fprintln(w, "Don't forget to modify variables to your .gitlab-ci.yml file:")
				fmt.Fprintf(w, "\t%s\n", "- MODULES_MODULE_NAME")
				fmt.Fprintf(w, "\t%s\n", "- MODULES_REGISTRY")
				fmt.Fprintf(w, "\t%s\n", "- MODULES_MODULE_SOURCE")
				fmt.Fprintf(w, "\t%s\n", "- MODULES_MODULE_TAG")
				fmt.Fprintf(w, "\t%s\n", "- WERF_VERSION")
				fmt.Fprintf(w, "\t%s\n", "- BASE_IMAGES_VERSION")
			}

			w.Flush()

			fmt.Print(buf.String())

			return nil
		},
	}

	lintCmd.Flags().AddFlagSet(flags.InitLintFlagSet())
	bootstrapCmd.Flags().AddFlagSet(flags.InitBootstrapFlagSet())

	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.PersistentFlags().AddFlagSet(flags.InitDefaultFlagSet())

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

	// Process all directories and combine results
	if err := runLintMultiple(dirs); err != nil {
		os.Exit(1)
	}
}

func runLintMultiple(dirs []string) error {
	var hasErrors bool

	// Process each directory separately
	for _, dir := range dirs {
		expandedDir, err := fsutils.ExpandDir(dir)
		if err != nil {
			logger.ErrorF("Error expanding directory %s: %v", dir, err)
			continue
		}

		logger.InfoF("Processing directory: %s", expandedDir)

		// Run lint for this directory as a separate execution
		if err := runLint(expandedDir); err != nil {
			logger.ErrorF("Error processing directory %s: %v", expandedDir, err)
			hasErrors = true
			// Continue processing other directories even if one fails
		}
	}

	if hasErrors {
		return fmt.Errorf("critical errors found")
	}

	return nil
}
