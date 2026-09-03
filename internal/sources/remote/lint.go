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

// Package remote lints a module as it was published, rather than as it sits in a
// working tree: it pulls the two images a release produces and runs the scopes that
// belong to them.
package remote

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
	"github.com/deckhouse/dmt/internal/modules"
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

// Source reads a module from the two images published under an image path, e.g.
// registry.example.com/my-module:v0.0.1.
type Source struct {
	client     registry.Client
	tag        string
	moduleName string

	// dirs are the extraction directories, removed by Close.
	dirs []string
}

var _ manager.Source = (*Source)(nil)

// NewSource resolves the image path up front, so an unusable reference fails before
// the run prints anything.
func NewSource(imagePath string, opts *Options) (*Source, error) {
	repository, tag, err := cutTagFromImagePath(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to cut tag from image path: %w", err)
	}

	return &Source{
		client:     newRegistryClient(repository, opts.Login, opts.Password),
		tag:        tag,
		moduleName: path.Base(repository),
	}, nil
}

// ConfigDir is the caller's working directory: the image ships no .dmtlint.yaml, so
// severities come from the config next to whoever started the run — read from its
// `remote.bundle` and `remote.release` sections rather than the ones the source tree
// uses.
func (s *Source) ConfigDir() string {
	return "."
}

func (s *Source) Scopes() []scopes.Scope {
	return []scopes.Scope{scopes.Bundle, scopes.Release}
}

// Close removes the extraction directories. It runs after the findings are printed,
// which is why every remote rule reports a module-relative path.
func (s *Source) Close() {
	for _, dir := range s.dirs {
		os.RemoveAll(dir)
	}
}

// Targets pulls and unpacks both images. Both are attempted before either error is
// returned: a registry failure on one must not cost the caller what the other holds.
func (s *Source) Targets(
	ctx context.Context,
	cfg *config.RootConfig,
	_ *errors.LintRuleErrorsList,
) ([]manager.Target, error) {
	bundle, bundleErr := s.target(ctx, s.client, scopes.Bundle, cfg)
	release, releaseErr := s.target(ctx, s.client.WithSegment(releaseSegment), scopes.Release, cfg)

	targets := make([]manager.Target, 0, 2)

	for _, t := range []*manager.Target{bundle, release} {
		if t != nil {
			targets = append(targets, *t)
		}
	}

	return targets, stderrors.Join(bundleErr, releaseErr)
}

// target pulls one image and unpacks it into a module. Which linters the scope runs
// over it, and which of their rules, is the scope's business.
func (s *Source) target(
	ctx context.Context,
	client registry.Client,
	scope scopes.Scope,
	cfg *config.RootConfig,
) (*manager.Target, error) {
	image, err := client.GetImage(ctx, s.tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s image: %w", scope, err)
	}

	dir, err := extractImage(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("failed to extract %s image: %w", scope, err)
	}

	s.dirs = append(s.dirs, dir)

	return &manager.Target{
		Module: modules.NewRemoteModule(dir, s.moduleName, scope.Settings(cfg)),
		Scope:  scope,
		// Both images are the same module, so the summary counts one.
		ModuleID: s.moduleName,
		ObjectID: string(scope),
	}, nil
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

	// Without an empty default registry, name invents one: "deckhouse/my-module:v0.0.1"
	// becomes index.docker.io/deckhouse/my-module, and a bare "registry.example.com:5000"
	// becomes index.docker.io/library/registry.example.com with "5000" as its tag. The
	// pull would then send --login/--password to Docker Hub, so the host is required
	// here rather than guessed.
	ref, err := name.ParseReference(imagePath, name.WithDefaultTag(""), name.WithDefaultRegistry(""))
	if err != nil {
		return "", "", fmt.Errorf("failed to parse image path: %w", err)
	}

	if ref.Context().RegistryStr() == "" {
		return "", "", fmt.Errorf("registry not found in image path, expected <registry>/<repo>:<tag>")
	}

	tag := ref.Identifier()
	if tag == "" {
		return "", "", fmt.Errorf("tag not found in image path")
	}

	return ref.Context().Name(), tag, nil
}
