package openapienum

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/deckhouse/dmt/pkg/config"
)

var (
	arrayPathRegex = regexp.MustCompile(`[\d+]`)
)

type EnumValidator struct {
	cfg *config.OpenAPIEnumSettings

	excludes map[string]struct{}
}

func NewEnumValidator(cfg *config.OpenAPIEnumSettings) EnumValidator {
	keyExcludes := make(map[string]struct{})

	for _, exc := range cfg.EnumFileExcludes["*"] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	return EnumValidator{
		cfg:      cfg,
		excludes: keyExcludes,
	}
}

func (EnumValidator) GetKey() string {
	return "enum"
}

func (en EnumValidator) Run(moduleName, absoluteKey string, value any) error {
	parts := strings.Split(absoluteKey, ".")
	if parts[len(parts)-1] != "enum" {
		return nil
	}

	en.excludes = make(map[string]struct{})
	for _, exc := range en.cfg.EnumFileExcludes[moduleName] {
		en.excludes[exc+".enum"] = struct{}{}
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
			res = errors.Join(res, fmt.Errorf("enum '%s' is invalid: %w", enumKey, err))
		}
	}

	return res
}

func validateEnumValue(value string) error {
	if value == "" {
		return nil
	}

	vv := []rune(value)
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
