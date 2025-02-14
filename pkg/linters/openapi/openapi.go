package openapi

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/openapi/rules"
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

	// check openAPI files
	openAPIFiles := fsutils.GetFiles(m.GetPath(), true, filterOpenAPIfiles)

	enumValidator := rules.NewEnumRule(o.cfg, m.GetPath())
	haValidator := rules.NewHARule(o.cfg, m.GetPath())

	for _, file := range openAPIFiles {
		enumValidator.Run(file, errorLists)
		haValidator.Run(file, errorLists)
	}

	// check only CRDs files
	crdFiles := fsutils.GetFiles(m.GetPath(), true, filterCRDsfiles)
	crdValidator := rules.NewDeckhouseCRDsRule(m.GetPath())
	keyValidator := rules.NewKeysRule(o.cfg, m.GetPath())
	for _, file := range crdFiles {
		enumValidator.Run(file, errorLists)
		haValidator.Run(file, errorLists)
		keyValidator.Run(file, errorLists)
		crdValidator.Run(file, errorLists)
	}
}

func (o *OpenAPI) Name() string {
	return o.name
}

func (o *OpenAPI) Desc() string {
	return o.desc
}

var openapiYamlRegex = regexp.MustCompile(`^openapi/.*\.ya?ml$`)

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

var crdsYamlRegex = regexp.MustCompile(`^crds/.*\.ya?ml$`)

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
