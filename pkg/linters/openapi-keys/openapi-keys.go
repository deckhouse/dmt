package openapikeys

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

// Enum linter
type Enum struct {
	name, desc string
	cfg        *config.OpenAPIKeysSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Enum {
	return &Enum{
		name:      "openapi-keys",
		desc:      "Linter will check openapi keys values is correct",
		cfg:       &cfg.LintersSettings.OpenAPIKeys,
		ErrorList: errorList.WithLinterID("openapi-keys").WithMaxLevel(cfg.LintersSettings.Conversions.Impact),
	}
}

func (e *Enum) Run(m *module.Module) *errors.LintRuleErrorsList {
	errorLists := e.ErrorList.WithModule(m.GetName())
	files := fsutils.GetFiles(m.GetPath(), true, filterFiles)
	parser := NewKeyValidator(e.cfg)
	for _, file := range files {
		if err := openapi.Parse(parser.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("openAPI file is not valid: %s", err)
		}
	}

	return errorLists
}

func (e *Enum) Name() string {
	return e.name
}

func (e *Enum) Desc() string {
	return e.desc
}

var r = regexp.MustCompile(`.*crds/.*\.ya?ml$`)

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
