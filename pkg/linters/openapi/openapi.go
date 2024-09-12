package openapi

import (
	"context"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/module"
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

func (o *OpenAPI) Run(_ context.Context, m *module.Module) (errors.LintRuleErrorsList, error) {
	apiFiles, err := GetOpenAPIYAMLFiles(m.Path)
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC, o.cfg)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
			rootPath: m.Path,
		}
	}
	close(filesC)

	var result errors.LintRuleErrorsList
	for res := range resultC {
		if res.validationError != nil {
			result.Add(errors.LintRuleError{
				Text:     res.validationError.Error(),
				ID:       "openapi",
				ObjectID: strings.TrimPrefix(res.filePath, m.Path),
				Value:    res.validationError,
			})
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
