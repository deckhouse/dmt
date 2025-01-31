package openapi

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *config.OpenAPISettings
}

func New(cfg *config.OpenAPISettings) *OpenAPI {
	return &OpenAPI{
		name: "openapi",
		desc: "OpenAPI will check all openapi files in the module",
		cfg:  cfg,
	}
}

func (o *OpenAPI) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("openapi", m.GetName())
	if m.GetPath() == "" {
		return result
	}
	apiFiles, err := GetOpenAPIYAMLFiles(m.GetPath())
	if err != nil {
		result.AddValue(err.Error(), "failed to get openapi files in `%s` module", m.GetName())
		return result
	}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC, o.cfg)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			moduleName: m.GetName(),
			filePath:   apiFile,
			rootPath:   m.GetPath(),
		}
	}
	close(filesC)

	for res := range resultC {
		if res.validationError != nil {
			result.WithObjectID(res.filePath).
				AddValue(res.validationError.Error(), "errors in `%s` module", m.GetName())
		}
	}

	return result
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}
