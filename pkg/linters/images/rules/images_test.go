package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/dmt/pkg/config"
)

func TestWerfFileLint(t *testing.T) {
	Cfg = new(config.ImageSettings)

	t.Run("check werf file with multiply images", func(t *testing.T) {
		lerr := lintOneDockerfile("testmodule", "testdata/werf.inc.yaml", "testdata")
		assert.Len(t, lerr, 3)
		for _, l := range lerr.GetList() {
			switch l.Value {
			case "$.Images.BASE_ALT_P11":
				assert.Contains(t, l.Text, "Use `from:` or `fromImage:` and `final: false` directives instead of `artifact:`")
			case "$.Images.FOOBAR", "$.Images.FOOBAZ":
				assert.Contains(t, l.Text, "`from:` parameter should be one of our BASE_DISTROLESS images")
			}
		}
	})
}
