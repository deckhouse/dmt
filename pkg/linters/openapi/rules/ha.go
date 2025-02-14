package rules

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type HARule struct {
	cfg *config.OpenAPISettings
	pkg.RuleMeta
	pkg.StringRule
	rootPath string
}

func NewHARule(cfg *config.OpenAPISettings, rootPath string) *HARule {
	return &HARule{
		cfg: cfg,
		RuleMeta: pkg.RuleMeta{
			Name: "high-availability",
		},
		StringRule: pkg.StringRule{
			ExcludeRules: cfg.HAAbsoluteKeysExcludes.Get(),
		},
		rootPath: rootPath,
	}
}

func (e *HARule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(e.GetName())

	shortPath, _ := filepath.Rel(e.rootPath, path)
	haValidator := newHAValidator(e.StringRule)

	if err := openapi.Parse(haValidator.run, path); err != nil {
		errorList.WithFilePath(shortPath).Errorf("openAPI file is not valid:\n%s", err)
	}
}

type haValidator struct {
	rule pkg.StringRule
}

func newHAValidator(rule pkg.StringRule) haValidator {
	return haValidator{
		rule: rule,
	}
}

func (v *haValidator) run(absoluteKey string, value any) error {
	if !v.rule.Enabled(absoluteKey) {
		return nil
	}

	// Ignore key inside a deep structure, like properties.internal.spec.xxx
	if absoluteKey != "properties.highAvailability" {
		return nil
	}

	parts := strings.Split(absoluteKey, ".")
	key := parts[len(parts)-1]
	if key != "highAvailability" && key != "https" {
		return nil
	}

	m := make(map[any]any)
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map {
		return fmt.Errorf("possible Bug? Have to be a map. Type: %s, Value: %s", reflect.TypeOf(value), value)
	}

	for _, key := range rv.MapKeys() {
		v := rv.MapIndex(key)
		m[key.Interface()] = v.Interface()
	}

	for key := range m {
		if key == "default" {
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
