package module

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/mitchellh/hashstructure/v2"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/d8-lint/internal/helm"
	"github.com/deckhouse/d8-lint/internal/storage"
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

	var docBytes []byte

	for path, bigFile := range files {
		scanner := bufio.NewScanner(strings.NewReader(bigFile))
		scanner.Split(SplitAt("---"))

		for scanner.Scan() {
			var node map[string]any
			docBytes = scanner.Bytes()

			err = yaml.Unmarshal(docBytes, &node)
			if err != nil {
				return fmt.Errorf(manifestErrorMessage, err)
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
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
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

		return
	}
}

const (
	manifestErrorMessage = `manifest unmarshal: %v`
)
