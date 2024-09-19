package helm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogTrimmer(t *testing.T) {
	testData := []byte(`
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...`)

	var buf bytes.Buffer

	wrapper := FilteredHelmWriter{Writer: &buf}

	_, err := wrapper.Write(testData)
	require.NoError(t, err)

	result := `
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...`
	require.Equal(t, buf.String(), result)
}
