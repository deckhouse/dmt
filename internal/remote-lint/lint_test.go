package remotelint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCutTagFromImagePath(t *testing.T) {
	// Simple successful case
	repository, tag, err := cutTagFromImagePath("registry.example.com/deckhouse/my-module:v0.0.1")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com/deckhouse/my-module", repository)
	require.Equal(t, "v0.0.1", tag)

	// Image with digest
	_, _, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module@sha256:1234567890")
	require.ErrorContains(t, err, "digest not supported")

	// Image without tag
	_, _, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module")
	require.ErrorContains(t, err, "tag not found in image path")

	// Registry with a port
	repository, tag, err = cutTagFromImagePath("registry.example.com:8080/deckhouse/my-module:v0.0.1")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com:8080/deckhouse/my-module", repository)
	require.Equal(t, "v0.0.1", tag)
}
