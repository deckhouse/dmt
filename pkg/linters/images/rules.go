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

package images

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ImagesDir = "images"
)

func IsExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

func (l *Images) ApplyImagesRules(m *module.Module, result *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	l.checkImageNamesInDockerFiles(m.GetName(), m.GetPath(), result)

	lintWerfFile(m.GetWerfFile(), result)

	return result
}
