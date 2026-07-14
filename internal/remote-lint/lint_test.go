package remotelint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCutTagFromImagePath(t *testing.T) {
	repository, tag, err := cutTagFromImagePath("registry.example.com/deckhouse/my-module:v0.0.1")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com/deckhouse/my-module", repository)
	require.Equal(t, "v0.0.1", tag)

	repository, tag, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module@sha256:1234567890")
	require.Contains(t, err.Error(), "digest not supported")
	require.Empty(t, repository)
	require.Empty(t, tag)

	repository, tag, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module")
	require.Contains(t, err.Error(), "tag not found in image path")
	require.Empty(t, repository)
	require.Empty(t, tag)
}
