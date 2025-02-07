package openapi

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "openapi"
)

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *config.OpenAPISettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      ID,
		desc:      "OpenAPI will check all openapi files in the module",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
	}
}

func (o *OpenAPI) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("openapi", m.GetName()).WithMaxLevel(o.cfg.Impact)
	if m.GetPath() == "" {
		return result
	}

	apiFiles, err := GetOpenAPIYAMLFiles(m.GetPath())
	if err != nil {
		result.WithValue(err.Error()).Add("failed to get openapi files in `%s` module", m.GetName())
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
				WithValue(res.validationError.Error()).Add("errors in `%s` module", m.GetName())
		}
	}

	result.CorrespondToMaxLevel()

	o.ErrorList.Merge(result)

	return result
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}
