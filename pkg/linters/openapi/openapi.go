package openapi

import (
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

type Module interface {
	GetName() string
	GetPath() string
}

func New(cfg *config.OpenAPISettings) *OpenAPI {
	return &OpenAPI{
		name: "openapi",
		desc: "OpenAPI will check all openapi files in the module",
		cfg:  cfg,
	}
}

func (o *OpenAPI) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
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
			path, _ := strings.CutPrefix(res.filePath, res.rootPath)
			result.Add(errors.NewLintRuleError(
				"openapi",
				path,
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
