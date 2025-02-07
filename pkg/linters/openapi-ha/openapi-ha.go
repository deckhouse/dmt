package openapiha

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// HA linter
type HA struct {
	name, desc string
	cfg        *config.OpenAPIHASettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *HA {
	return &HA{
		name:      "openapi-ha",
		desc:      "Linter will check openapi ha values is correct",
		cfg:       &cfg.LintersSettings.OpenAPIHA,
		ErrorList: errorList.WithLinterID("openapi-ha").WithMaxLevel(cfg.LintersSettings.Conversions.Impact),
	}
}

func (e *HA) Run(m *module.Module) *errors.LintRuleErrorsList {
	errorLists := e.ErrorList.WithModule(m.GetName())
	files := fsutils.GetFiles(m.GetPath(), true, filterFiles)
	parser := NewHAValidator(e.cfg)
	for _, file := range files {
		if err := openapi.Parse(parser.Run, file); err != nil {
			errorLists.WithValue(err).Errorf("openAPI file is not valid: %s", fsutils.Rel(m.GetPath(), file))
		}
	}

	return errorLists
}

func (e *HA) Name() string {
	return e.name
}

func (e *HA) Desc() string {
	return e.desc
}

var r = regexp.MustCompile(`.*(?:openapi|crds)/.*\.ya?ml$`)

func filterFiles(path string) bool {
	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}
	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return r.MatchString(path)
}
