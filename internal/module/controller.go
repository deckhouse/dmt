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
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/mitchellh/hashstructure/v2"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/helm"
	"github.com/deckhouse/dmt/internal/storage"
)

var (
	renderedTemplatesHash = sync.Map{}
)

func RunRender(m *Module, values chartutil.Values, objectStore *storage.UnstructuredObjectStore) error {
	var renderer helm.Renderer
	renderer.Name = m.GetName()
	renderer.Namespace = m.GetNamespace()
	renderer.LintMode = true

	files, err := renderer.RenderChartFromRawValues(m.GetChart(), values)
	if err != nil {
		return fmt.Errorf("helm chart render: %w", err)
	}

	hash, err := hashstructure.Hash(files, hashstructure.FormatV2, nil)
	if err != nil {
		return fmt.Errorf("helm chart render: %w", err)
	}

	if _, ok := renderedTemplatesHash.Load(hash); ok {
		return nil
	}

	defer renderedTemplatesHash.Store(hash, struct{}{})

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
				return fmt.Errorf("helm chart object already exists: %w", err)
			}
		}
	}

	return nil
}

func SplitAt(substring string) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		// Return nothing if at end of file and no data passed
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Find the index of the input of the separator substring
		if i := bytes.Index(data, []byte(substring)); i >= 0 {
			return i + len(substring), data[0:i], nil
		}

		// If at end of file with data return the data
		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	}
}

const (
	manifestErrorMessage = `manifest (%q) unmarshal: %v`
)
