package openapi

import (
	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// OpenAPI linter
type OpenAPI struct {
	name string
	cfg  *config.OpenAPISettings
}

func Run(m *module.Module) {
	o := &OpenAPI{
		name: "openapi",
		cfg:  &config.Cfg.LintersSettings.OpenAPI,
	}

	logger.DebugF("Running linter `%s` on module `%s`", o.name, m.GetName())

	lintError := errors.NewError(o.name, m.GetName())
	if m.GetPath() == "" {
		return
	}

	apiFiles, err := GetOpenAPIYAMLFiles(m.GetPath())
	if err != nil {
		lintError.WithValue(err.Error()).Add("failed to get openapi files in `%s` module", m.GetName())
		return
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
			lintError.WithObjectID(res.filePath).
				WithValue(res.validationError.Error()).Add("errors in `%s` module", m.GetName())
		}
	}
}
