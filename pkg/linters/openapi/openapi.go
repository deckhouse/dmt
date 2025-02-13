package openapi

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

// OpenAPI linter
type OpenAPI struct {
	name, desc string
	cfg        *config.OpenAPISettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *OpenAPI {
	return &OpenAPI{
		name:      "openapi",
		desc:      "Linter will check openapi values is correct",
		cfg:       &cfg.LintersSettings.OpenAPI,
		ErrorList: errorList.WithLinterID("openapi").WithMaxLevel(cfg.LintersSettings.OpenAPI.Impact),
	}
}

func (o *OpenAPI) Run(m *module.Module) {
	errorLists := o.ErrorList.WithModule(m.GetName())

	// check openAPI and CRDs files
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

	enumValidator := NewEnumValidator(o.cfg)
	haValidator := NewHAValidator(o.cfg)

	for _, file := range openAPIFiles {
		if err := openapi.Parse(enumValidator.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("openAPI file is not valid:\n%s", err)
		}
		if err := openapi.Parse(haValidator.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("openAPI file is not valid:\n%s", err)
		}
	}

	// check only CRDs files
	crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
	KeyValidator := NewKeyValidator(o.cfg)
	for _, file := range crdFiles {
		if err := openapi.Parse(enumValidator.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("CRD file is not valid:\n%s", err)
		}
		if err := openapi.Parse(haValidator.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("CRD file is not valid:\n%s", err)
		}
		if err := openapi.Parse(KeyValidator.Run, file); err != nil {
			errorLists.WithFilePath(fsutils.Rel(m.GetPath(), file)).Errorf("CRD file is not valid: %s", err)
		}
		validateDeckhouseCRDS(m.GetName(), file, errorLists)
	}
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}

var openapiYamlRegex = regexp.MustCompile(`.*openapi/.*\.ya?ml$`)

func filterOpenAPIfiles(path string) bool {
	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}
	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return openapiYamlRegex.MatchString(path)
}

var crdsYamlRegex = regexp.MustCompile(`.*crds/.*\.ya?ml$`)

func filterCRDsfiles(path string) bool {
	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}
	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return crdsYamlRegex.MatchString(path)
}
