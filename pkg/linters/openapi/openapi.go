package openapi

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

const (
	ID = "openapi"
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

func (o *OpenAPI) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	if m.GetPath() == "" {
		return errors.LintRuleErrorsList{}, nil
	}
	apiFiles, err := GetOpenAPIYAMLFiles(m.GetPath())
	if err != nil {
		return errors.LintRuleErrorsList{}, err
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

	var result errors.LintRuleErrorsList
	for res := range resultC {
		if res.validationError != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				res.filePath,
				m.GetName(),
				res.validationError,
				"errors in `%s` module",
				m.GetName(),
			))
		}
	}

	return result, nil
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}
