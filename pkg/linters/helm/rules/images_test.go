package rules

import (
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWerfFileLint(t *testing.T) {
	Cfg = new(config.HelmSettings)

	t.Run("File where final: false for non-distroless image", func(t *testing.T) {
		lerr := lintOneDockerfileOrWerfYAML("testmodule", "testdata/with_final_false/werf.inc.yaml", "testdata/with_final_false")
		assert.Nil(t, lerr)
	})

	t.Run("File where final: true for non-distroless image", func(t *testing.T) {
		lerr := lintOneDockerfileOrWerfYAML("testmodule", "testdata/with_final_true/werf.inc.yaml", "testdata/with_final_true")
		assert.NotNil(t, lerr)
		assert.Contains(t, lerr.Text, "should be one of our BASE_DISTROLESS images")
		assert.Contains(t, lerr.Value, "$.Images.FOOBAR")
	})

	t.Run("File with `final` flag not set", func(t *testing.T) {
		lerr := lintOneDockerfileOrWerfYAML("testmodule", "testdata/with_empty_final/werf.inc.yaml", "testdata/with_empty_final")
		assert.NotNil(t, lerr)
		assert.Contains(t, lerr.Text, "should be one of our BASE_DISTROLESS images")
		assert.Contains(t, lerr.Value, "$.Images.FOOBAR")
	})
}
