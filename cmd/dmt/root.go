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
	"log/slog"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/dmt/internal/bootstrap"
	"github.com/deckhouse/dmt/internal/flags"
	"github.com/deckhouse/dmt/internal/fsutils"
	remotelint "github.com/deckhouse/dmt/internal/remote-lint"
	"github.com/deckhouse/dmt/internal/render"
	"github.com/deckhouse/dmt/internal/test"
	"github.com/deckhouse/dmt/internal/version"
	"github.com/deckhouse/dmt/pkg/config"
)

var kebabCaseRegex = regexp.MustCompile(`^([a-z][a-z0-9]*)(-[a-z0-9]+)*$`)

func execute() {
	rootCmd := &cobra.Command{
		Use:   "dmt",
		Short: "Deckhouse module tools",
		Long:  `It's a swiss knife for everyone, who want's create, maintain or use deckhouse modules.`,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		Version: version.Version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			// TODO: move to separate package
			flags.Version = version.Version
			lvl := log.LogLevelFromStr(flags.LogLevel)
			logger := log.NewLogger(
				log.WithLevel(slog.Level(lvl)),
				log.WithHandlerType(log.TextHandlerType),
			)
			log.SetDefault(logger)
		},
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

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Tests for Deckhouse modules",
		Long:  `Run tests on module conversions and other components`,
	}

	conversionsCmd := &cobra.Command{
		Use:          "conversions [module-path]",
		Short:        "Test module conversion specifications",
		Long:         `Validates that conversion specifications match the OpenAPI config versions.`,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			var dir = "."
			if len(args) > 0 {
				dir = args[0]
			}

			return runTests(dir, test.WithTesters("conversions"))
		},
	}

	var updateSnapshots bool

	templatesCmd := &cobra.Command{
		Use:   "templates [module-path]",
		Short: "Test module templates against golden snapshots",
		Long: `Renders module templates with the values provided under each module's
'templates-tests/<case>/values.yaml' and compares the result against the
committed 'expected.yaml' snapshot. Use --update to (re)generate snapshots.`,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			var dir = "."
			if len(args) > 0 {
				dir = args[0]
			}

			return runTests(dir,
				test.WithTesters("templates"),
				test.WithUpdateSnapshots(updateSnapshots),
			)
		},
	}
	templatesCmd.Flags().BoolVar(&updateSnapshots, "update", false,
		"update (regenerate) golden snapshots instead of comparing against them")

	testCmd.AddCommand(conversionsCmd)
	testCmd.AddCommand(templatesCmd)

	var renderOutput string

	renderCmd := &cobra.Command{
		Use:   "render [module-path]",
		Short: "Render Deckhouse module templates",
		Long: `Finds all modules under the given path (including subdirectories) and renders
each module's templates from its 'templates' directory using values generated
from the module's openapi schemas ('openapi/config-values.yaml' and
'openapi/values.yaml', if present).

By default the output is written into a 'rendered' directory at each module
root. When --output is set, the output is instead written into
'<output>/rendered/<module-name>/<edition>/', where the module name is taken
from the module's 'module.yaml' and editions follow the
'openapi/values_<edition>.yaml' convention (with 'default' for
'openapi/values.yaml').`,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			var dir = "."
			if len(args) > 0 {
				dir = args[0]
			}

			return render.Render(dir, renderOutput)
		},
	}
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "",
		"directory to write the rendered output into (created if absent; a 'rendered' subdirectory is created inside it)")

	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(renderCmd)
	rootCmd.Flags().AddFlagSet(flags.InitDefaultFlagSet())

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runTests(dir string, opts ...test.Option) error {
	cfg, err := config.NewDefaultRootConfig(dir)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	expandedDir, err := fsutils.ExpandDir(dir)
	if err != nil {
		return fmt.Errorf("failed to expand directory: %w", err)
	}

	manager, err := test.NewManager(expandedDir, cfg, opts...)
	if err != nil {
		return fmt.Errorf("failed to create test manager: %w", err)
	}

	manager.Run()
	manager.PrintResult()

	if manager.HasCriticalErrors() {
		os.Exit(1)
	}

	return nil
}

func lintCmdFunc(cmd *cobra.Command, args []string) {
	if flags.Remote != "" {
		opts := &remotelint.RemoteLintOptions{
			Login:    flags.RemoteLogin,
			Password: flags.RemotePassword,
		}
		if err := remotelint.RunRemoteLint(cmd.Context(), flags.Remote, opts); err != nil {
			log.Error("Error running remote lint", log.Err(err))
			os.Exit(1)
		}

		return
	}

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
			log.Error("Error expanding directory", slog.String("dir", dir), log.Err(err))
			continue
		}

		log.Info("Processing directory", slog.String("directory", expandedDir))

		// Run lint for this directory as a separate execution
		if err := runLint(expandedDir); err != nil {
			log.Error("Error processing directory", slog.String("directory", expandedDir), log.Err(err))

			hasErrors = true
			// Continue processing other directories even if one fails
		}
	}

	if hasErrors {
		return fmt.Errorf("critical errors found")
	}

	return nil
}
