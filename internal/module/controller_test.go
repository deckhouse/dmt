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

package module

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const configMapWithYAMLStream = `apiVersion: v1
kind: ConfigMap
metadata:
  name: xxx
data:
  apply-on-startup.yaml: |2
    kind: token
    version: v2
    metadata:
      name: teleport-proxy
    ---
    kind: token
    version: v2
    metadata:
      name: teleport-appservice`

const serviceManifest = `apiVersion: v1
kind: Service
metadata:
  name: xxx`

func TestSplitYAMLDocumentsWithBlockScalarIndentIndicator(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantDocs []string
	}{
		{
			name:     "keeps separator inside block scalar",
			content:  configMapWithYAMLStream,
			wantDocs: []string{configMapWithYAMLStream},
		},
		{
			name:     "splits after block scalar",
			content:  joinYAMLDocuments(configMapWithYAMLStream, serviceManifest),
			wantDocs: []string{configMapWithYAMLStream, serviceManifest},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs := splitYAMLDocuments(tt.content)
			require.Equal(t, tt.wantDocs, docs)
			requireValidYAMLDocuments(t, docs)
		})
	}
}

func joinYAMLDocuments(docs ...string) string {
	return strings.Join(docs, "\n---\n")
}

func requireValidYAMLDocuments(t *testing.T, docs []string) {
	t.Helper()

	for _, doc := range docs {
		var node map[string]any
		require.NoError(t, yaml.UnmarshalStrict([]byte(doc), &node))
	}
}
