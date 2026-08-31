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

// Package remotelint lints a module as it was published, rather than as it sits in a
// working tree: it pulls the two images a release produces and runs the scopes that
// belong to them.
package remotelint

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/pkg/registry"

	"github.com/deckhouse/dmt/internal/manager"
	"github.com/deckhouse/dmt/internal/metrics"
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/scopes"
)

// releaseSegment is the repository segment a module's release image sits under:
// the bundle is <repo>:<tag> and the release is <repo>/release:<tag>.
const releaseSegment = "release"

type Options struct {
	// Login is the username to use for the registry, e.g. license-token.
	Login string
	// Password is the password to use for the registry.
	Password string
}

// Run lints the bundle and release images published under imagePath, e.g.
// registry.example.com/my-module:v0.0.1.
func Run(ctx context.Context, imagePath string, opts *Options) error {
	repository, tag, err := cutTagFromImagePath(imagePath)
	if err != nil {
		return fmt.Errorf("failed to cut tag from image path: %w", err)
	}

	// The image ships no .dmtlint.yaml, so severities come from the config next to
	// the caller — the same file a local run would read.
	cfg, err := config.NewDefaultRootConfig(".")
	if err != nil {
		return fmt.Errorf("failed to parse default root config: %w", err)
	}

	// init metrics storage, should be done before printing results, as PrintResult
	// reports per-error metrics through the shared metrics client.
	metrics.GetClient(".")

	client := newRegistryClient(repository, opts.Login, opts.Password)
	moduleName := path.Base(repository)

	level := pkg.Error
	errorList := errors.NewLintRuleErrorsList().WithMaxLevel(&level)

	// Both images are linted before anything is printed: a registry failure on one
	// must not swallow what the other already found.
	pullErr := stderrors.Join(
		lintImage(ctx, client, tag, scopes.Bundle, moduleName, cfg, errorList),
		lintImage(ctx, client.WithSegment(releaseSegment), tag, scopes.Release, moduleName, cfg, errorList),
	)

	manager.PrintResult(errorList)

	if pullErr != nil {
		return pullErr
	}

	if errorList.ContainsErrors() {
		return stderrors.New("critical errors found")
	}

	return nil
}

// lintImage pulls one image, unpacks it and runs the scope that belongs to it. Which
// linters that scope runs, and which of their rules, is the scope's business — this
// only supplies the unpacked module.
func lintImage(
	ctx context.Context,
	client registry.Client,
	tag string,
	scope scopes.Scope,
	moduleName string,
	cfg *config.RootConfig,
	errorList *errors.LintRuleErrorsList,
) error {
	image, err := client.GetImage(ctx, tag)
	if err != nil {
		return fmt.Errorf("failed to get %s image: %w", scope, err)
	}

	dir, err := extractImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to extract %s image: %w", scope, err)
	}
	defer os.RemoveAll(dir)

	m, err := modules.NewRemoteModule(dir, moduleName, cfg)
	if err != nil {
		return fmt.Errorf("failed to read config for %s image: %w", scope, err)
	}

	for _, linter := range scope.Linters(m, errorList.WithObjectID(string(scope))) {
		linter.Lint(ctx)
	}

	return nil
}

// cutTagFromImagePath splits an image path into repository and tag, turning
// "registry.example.com/my-module:v0.0.1" into "registry.example.com/my-module" and
// "v0.0.1".
func cutTagFromImagePath(imagePath string) (string, string, error) {
	// The release image is addressed by the same tag under a sibling repository, and a
	// digest names one manifest only — there is no tag to carry over to it.
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
