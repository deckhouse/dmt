package remotelint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/pkg/registry/client"

	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters"
	"github.com/deckhouse/dmt/pkg/linters/docs"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
)

type RemoteLintOptions struct {
	Config *config.RootConfig
	// Login is the username to use for the registry e.g. license-token
	Login string
	// Password is the password to use for the registry
	Password string
}

// RunRemoteLint runs the remote linting for the given registry image and options
// RegistryPath is the path to the image e.g. registry.example.com/deckhouse/deckhouse:latest
func RunRemoteLint(ctx context.Context, imagePath string, opts *RemoteLintOptions) error {
	registryPath, tag, err := cutTagFromImagePath(imagePath)
	if err != nil {
		return fmt.Errorf("failed to cut tag from image path: %w", err)
	}

	client := initRegistryClient(registryPath, opts.Login, opts.Password)

	// init metrics storage, should be done before printing results, as
	// PrintResult reports per-error metrics through the shared metrics client.
	metrics.GetClient(".")

	level := pkg.Error
	errorList := errors.NewLintRuleErrorsList().WithMaxLevel(&level)

	cfg, err := parseConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	err = lintBundle(ctx, client, tag, cfg, errorList)
	if err != nil {
		return fmt.Errorf("failed to lint bundle: %w", err)
	}

	err = lintRelease(ctx, client, tag, cfg, errorList)
	if err != nil {
		// print result of successful bundle linting
		manager.PrintResult(errorList)
		return fmt.Errorf("failed to lint release: %w", err)
	}

	manager.PrintResult(errorList)

	if errorList.ContainsErrors() {
		return fmt.Errorf("critical errors found")
	}

	return nil
}

func lintBundle(ctx context.Context, client *client.Client, tag string, cfg *pkg.LintersSettings, errorList *errors.LintRuleErrorsList) error {
	image, err := client.GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	tempDir, err := ExtractImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}
	defer os.RemoveAll(tempDir)

	os.RemoveAll(filepath.Join(tempDir, "docs")) // debug: remove docs directory

	bundleLinters := buildBundleLinters(cfg, errorList.WithObjectID("bundle"))

	for _, linter := range bundleLinters {
		cfg := &linters.LinterConfig{
			Name:      client.GetRegistry(),
			Namespace: "bundle",
			Path:      tempDir,
		}
		linter.RunRemote(cfg)
	}

	return nil
}

func lintRelease(ctx context.Context, client *client.Client, tag string, cfg *pkg.LintersSettings, errorList *errors.LintRuleErrorsList) error {
	image, err := client.WithSegment("release").GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get release image: %w", err)
	}

	tempDir, err := ExtractImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to extract release image: %w", err)
	}
	defer os.RemoveAll(tempDir)

	releaseLinters := buildReleaseLinters(cfg, errorList.WithObjectID("release"))

	for _, linter := range releaseLinters {
		cfg := &linters.LinterConfig{
			Name:      client.GetRegistry(),
			Namespace: "release",
			Path:      tempDir,
		}
		linter.RunRemote(cfg)
	}

	return nil
}

// returns repository path and tag from the image path
// turns strings like "registry.example.com/my-module:v0.0.1" into "registry.example.com/my-module" and "v0.0.1"
func cutTagFromImagePath(imagePath string) (string, string, error) {
	// if digest was provided we can't know the release tag in future steps, so we can't pull it
	if strings.Contains(imagePath, "@") {
		return "", "", fmt.Errorf("digest not supported")
	}

	ref, err := name.ParseReference(imagePath, name.WithDefaultTag(""))
	if err != nil {
		return "", "", fmt.Errorf("failed to parse image path: %w", err)
	}

	tag := ref.Identifier()
	if tag == "" {
		return "", "", fmt.Errorf("tag not found in image path")
	}

	return ref.Context().Name(), tag, nil
}

func buildBundleLinters(cfg *pkg.LintersSettings, errorList *errors.LintRuleErrorsList) []linters.RemoteLinter {
	return []linters.RemoteLinter{
		docs.New(&cfg.Documentation, errorList.WithMaxLevel(cfg.Documentation.Impact)),
	}
}

func buildReleaseLinters(cfg *pkg.LintersSettings, errorList *errors.LintRuleErrorsList) []linters.RemoteLinter {
	return []linters.RemoteLinter{
		moduleLinter.New(&cfg.Module, errorList.WithMaxLevel(cfg.Module.Impact)),
	}
}

// parsing config from .dmtlint.yaml file
func parseConfig() (*pkg.LintersSettings, error) {
	rootConfig, err := config.NewDefaultRootConfig(".")
	if err != nil {
		return nil, fmt.Errorf("failed to parse default root config: %w", err)
	}

	// Load module config
	cfg := &config.ModuleConfig{}
	if err := config.NewLoader(cfg, ".").Load(); err != nil {
		return nil, fmt.Errorf("can not parse module config: %w", err)
	}

	cfg.LintersSettings.MergeGlobal(&rootConfig.GlobalSettings.Linters)

	return module.RemapLinterSettings(&cfg.LintersSettings, &rootConfig.GlobalSettings.Linters), nil
}
