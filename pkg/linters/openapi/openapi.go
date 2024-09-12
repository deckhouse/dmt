package openapi

import (
	"context"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/module"
)

// OpenAPI linter
type OpenAPI struct{}

func New() *OpenAPI {
	return &OpenAPI{}
}

func (*OpenAPI) Run(_ context.Context, m *module.Module) (errors.LintRuleErrorsList, error) {
	apiFiles, err := GetOpenAPIYAMLFiles(m.Path)
	if err != nil {
		return errors.LintRuleErrorsList{}, err
	}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

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

	return errors.LintRuleErrorsList{}, nil
}

func (*OpenAPI) Name() string {
	return "OpenAPI Linter"
}

func (*OpenAPI) Desc() string {
	return "OpenAPI will check all openapi files in the module"
}
