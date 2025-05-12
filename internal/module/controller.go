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

package module

import (
	"errors"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/helm"
	"github.com/deckhouse/dmt/internal/storage"
	dmtErrors "github.com/deckhouse/dmt/pkg/errors"
)

func RunRender(m *Module, values chartutil.Values, objectStore *storage.UnstructuredObjectStore, errorList *dmtErrors.LintRuleErrorsList) error {
	var renderer helm.Renderer
	renderer.Name = m.GetName()
	renderer.Namespace = m.GetNamespace()
	renderer.LintMode = true

	files, err := renderer.RenderChartFromRawValues(m.GetChart(), values)
	if err != nil {
		return fmt.Errorf("helm chart render: %w", err)
	}

	var resultErr error
	for path, bigFile := range files {
		for _, doc := range strings.Split(bigFile, "---") {
			docBytes := []byte(doc)
			if len(docBytes) == 0 {
				continue
			}
			node := make(map[string]any)
			err = yaml.UnmarshalStrict(docBytes, &node)
			if err != nil {
				return fmt.Errorf(manifestErrorMessage, strings.TrimPrefix(path, m.GetName()+"/"), err)
			}

			if len(node) == 0 {
				continue
			}

			err = objectStore.Put(path, node, docBytes)
			if err != nil {
				resultErr = errors.Join(resultErr, err)
				continue
			}
		}
	}

	if resultErr != nil {
		errorList.WithFilePath(m.GetPath()).WithModule(m.GetName()).
			WithValue(resultErr.Error()).Error("module contains duplicate objects")
	}

	return nil
}

const (
	manifestErrorMessage = `manifest (%q) unmarshal: %v`
)
