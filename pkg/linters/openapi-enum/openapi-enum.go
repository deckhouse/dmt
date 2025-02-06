package openapienum

import (
	"path/filepath"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Enum linter
type Enum struct {
	name, desc string
	cfg        *config.OpenAPIEnumSettings
}

func New(cfg *config.OpenAPIEnumSettings) *Enum {
	return &Enum{
		name: "openapi-enum",
		desc: "Probes will check openapi enum values is correct",
		cfg:  cfg,
	}
}

func (o *Enum) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("openapi-enum", m.GetName())

	files, err := fsutils.GetFiles(m.GetPath(), true, `.*(?:openapi|crds)/.*\.ya?ml$`)
	if err != nil {
		result.WithValue(err).Add("failed to get files in `%s` module", m.GetName())
		return result
	}

	parser := NewEnumValidator(o.cfg)
	for _, file := range files {
		data, err := openapi.GetFileYAMLContent(file)
		if err != nil {
			result.WithValue(err).Add("failed to get content of `%s`", file)
			continue
		}

		if err := openapi.Parse(parser, data); err != nil {
			f, _ := filepath.Rel(m.GetPath(), file)
			result.WithValue(err).Add("openAPI file is not valid: %s", f)
		}
	}

	return result
}

func (o *Enum) Name() string {
	return o.name
}

func (o *Enum) Desc() string {
	return o.desc
}
