package remotelint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/remote-linters/docs"
)

type RemoteLintOptions struct {
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

	image, err := client.GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	tempDir, err := ExtractImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to extract image: %w", err)
	}
	defer os.RemoveAll(tempDir)

	errorList := errors.NewLintRuleErrorsList() // .WithMaxLevel()

	linters, err := buildLinters(ctx, tempDir, errorList)
	if err != nil {
		return fmt.Errorf("failed to build linters: %w", err)
	}

	os.Remove(filepath.Join(tempDir, "docs", "README.md"))

	for _, linter := range linters {
		linter.Lint(ctx)
	}

	docs.PrintResult(errorList)

	return nil
}

// returns repository path and tag (or digest) from the image path
// turns strings like "registry.example.com/my-module:v0.0.1" into "registry.example.com/my-module" and "v0.0.1" (or sha256:aaa)
func cutTagFromImagePath(imagePath string) (string, string, error) {
	if parts := strings.Split(imagePath, "@"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	if parts := strings.Split(imagePath, ":"); len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("tag not found in image path: %s", imagePath)
}

type Linter interface {
	Lint(ctx context.Context)
}

func buildLinters(_ context.Context, path string, errorList *errors.LintRuleErrorsList) ([]Linter, error) {

	docsLinter := docs.NewLinter(docs.Config{Path: path}, errorList)

	return []Linter{
		docsLinter,
	}, nil
}
