/*
Copyright The Helm Authors.

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

package engine

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

// TestFilesAccess tests the access to chart files through templates
func TestFilesAccess(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "test-files",
			Version: "1.0.0",
		},
		Files: []*chart.File{
			{Name: "file1.txt", Data: []byte("contents of file1")},
			{Name: "file2.txt", Data: []byte("contents of file2")},
			{Name: "dir1/file3.txt", Data: []byte("contents of file3")},
			{Name: "nested/file4.txt", Data: []byte("line1\nline2\nline3")},
			{Name: "empty.txt", Data: []byte("")},
		},
		Templates: []*chart.File{
			{Name: "templates/getfile.yaml", Data: []byte(`file1: {{ .Files.Get "file1.txt" }}`)},
			{Name: "templates/getbytes.yaml", Data: []byte(`{{- $bytes := .Files.GetBytes "file1.txt" -}}
bytes_len: {{ len $bytes }}`)},
			{Name: "templates/glob.yaml", Data: []byte(`{{ range $path, $_ := .Files.Glob "*.txt" }}
- {{ $path }}
{{ end }}`)},
			{Name: "templates/lines.yaml", Data: []byte(`{{ range .Files.Lines "nested/file4.txt" }}
- {{ . }}
{{ end }}`)},
			{Name: "templates/asconfig.yaml", Data: []byte(`data: {{ .Files.AsConfig }}`)},
			{Name: "templates/assecrets.yaml", Data: []byte(`data: {{ .Files.AsSecrets }}`)},
		},
	}

	vals := map[string]any{}
	v, err := chartutil.CoalesceValues(c, vals)
	require.NoError(t, err)

	e := New()
	out, err := e.Render(c, v)
	require.NoError(t, err)

	// Test Get method
	assert.Contains(t, out["test-files/templates/getfile.yaml"], "file1: contents of file1")

	// Test GetBytes method
	assert.Contains(t, out["test-files/templates/getbytes.yaml"], "bytes_len: 17")

	// Test Glob method
	assert.Contains(t, out["test-files/templates/glob.yaml"], "- file1.txt")
	assert.Contains(t, out["test-files/templates/glob.yaml"], "- file2.txt")
	assert.Contains(t, out["test-files/templates/glob.yaml"], "- empty.txt")

	// Test Lines method
	assert.Contains(t, out["test-files/templates/lines.yaml"], "- line1")
	assert.Contains(t, out["test-files/templates/lines.yaml"], "- line2")
	assert.Contains(t, out["test-files/templates/lines.yaml"], "- line3")

	// Test AsConfig method
	assert.Contains(t, out["test-files/templates/asconfig.yaml"], "file1.txt: contents of file1")
	assert.Contains(t, out["test-files/templates/asconfig.yaml"], "file2.txt: contents of file2")

	// Test AsSecrets method
	expected1 := base64.StdEncoding.EncodeToString([]byte("contents of file1"))
	expected2 := base64.StdEncoding.EncodeToString([]byte("contents of file2"))
	assert.Contains(t, out["test-files/templates/assecrets.yaml"], "file1.txt: "+expected1)
	assert.Contains(t, out["test-files/templates/assecrets.yaml"], "file2.txt: "+expected2)
}
