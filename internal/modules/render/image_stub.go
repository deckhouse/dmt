/*
Copyright 2026 Flant JSC

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

package render

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// imageStubTemplate shadows deckhouse_lib_helm's image helpers so any image name
// resolves offline (see image_stub.tpl). Render injects it into templates/
// before each render.
//
//go:embed image_stub.tpl
var imageStubTemplate []byte

// imageStubFile is the injected override's name. The "_" prefix marks it a Helm
// partial (defines only, never rendered as a manifest); the name is distinctive so
// a leftover is unmistakable if a render is ever killed mid-flight.
const imageStubFile = "_dmt_image_stub.tpl"

// injectImageStub writes the image-resolution override (image_stub.tpl) into
// chartDir/templates/ so deckhouse_lib_helm's image helpers resolve any image name
// offline, then returns a cleanup func the caller must defer. The override is removed
// once the render is done, leaving the module's source untouched.
//
// A module without a templates/ directory ships no chart manifests (hooks-only
// modules) and so renders nothing that needs image resolution — injection is skipped.
func injectImageStub(chartDir string) (func(), error) {
	templatesDir := filepath.Join(chartDir, "templates")
	if _, err := os.Stat(templatesDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return func() {}, nil
		}

		return nil, fmt.Errorf("stat templates dir: %w", err)
	}

	stubPath := filepath.Join(templatesDir, imageStubFile)
	if err := os.WriteFile(stubPath, imageStubTemplate, 0o644); err != nil {
		// A partial write can leave the file behind; remove it so the module's
		// source is left untouched.
		_ = os.Remove(stubPath)

		return nil, fmt.Errorf("write image stub: %w", err)
	}

	return func() { _ = os.Remove(stubPath) }, nil
}
