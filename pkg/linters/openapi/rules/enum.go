/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rules

import (
	stdErrors "errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type EnumRule struct {
	cfg      *config.OpenAPISettings
	rootPath string
	pkg.RuleMeta
}

var (
	arrayPathRegex = regexp.MustCompile(`[\d+]`)
)

func NewEnumRule(cfg *config.OpenAPISettings, rootPath string) *EnumRule {
	return &EnumRule{
		cfg: cfg,
		RuleMeta: pkg.RuleMeta{
			Name: "enum",
		},
		rootPath: rootPath,
	}
}

func (e *EnumRule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(e.GetName())

	validator := newEnumValidator(e.cfg)

	shortPath, _ := filepath.Rel(e.rootPath, path)
	if err := openapi.Parse(validator.run, path); err != nil {
		errorList.WithFilePath(shortPath).Errorf("openAPI file is not valid:\n%s", err)
	}
}

type enumValidator struct {
	cfg *config.OpenAPISettings

	excludes map[string]struct{}
}

func newEnumValidator(cfg *config.OpenAPISettings) enumValidator {
	excludes := make(map[string]struct{})
	for _, exc := range cfg.OpenAPIExcludeRules.EnumFileExcludes {
		excludes[exc+".enum"] = struct{}{}
	}
	return enumValidator{
		cfg:      cfg,
		excludes: excludes,
	}
}

func (en enumValidator) run(absoluteKey string, value any) error {
	parts := strings.Split(absoluteKey, ".")
	if parts[len(parts)-1] != "enum" {
		return nil
	}

	if _, ok := en.excludes[absoluteKey]; ok {
		return nil
	}

	// check for slice path with wildcard
	index := arrayPathRegex.FindString(absoluteKey)
	if index != "" {
		wildcardKey := strings.ReplaceAll(absoluteKey, index, "*")
		if _, ok := en.excludes[wildcardKey]; ok {
			// excluding key with wildcard
			return nil
		}
	}

	values := value.([]any)
	enum := make([]string, 0, len(values))
	for _, val := range values {
		valStr, ok := val.(string)
		if !ok {
			continue // skip boolean flags
		}
		enum = append(enum, valStr)
	}

	err := validateEnumValues(absoluteKey, enum)

	return err
}

func validateEnumValues(enumKey string, values []string) error {
	var res error
	for _, value := range values {
		if err := validateEnumValue(value); err != nil {
			res = stdErrors.Join(res, fmt.Errorf("enum '%s' is invalid: %w", enumKey, err))
		}
	}

	return res
}

func validateEnumValue(value string) error {
	if value == "" {
		return nil
	}

	vv := []rune(value)
	if len(vv) == 0 {
		return nil
	}
	if unicode.IsLetter(vv[0]) && !unicode.IsUpper(vv[0]) {
		return fmt.Errorf("value '%s' must start with Capital letter", value)
	}

	for i, char := range vv {
		if unicode.IsLetter(char) {
			continue
		}
		if unicode.IsNumber(char) {
			continue
		}

		if char == '.' && i != 0 && unicode.IsNumber(vv[i-1]) {
			// permit dot into float numbers
			continue
		}

		// if rune is symbol/space/etc - it's invalid

		return fmt.Errorf("value: '%s' must be in CamelCase", value)
	}

	return nil
}
