package validators

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/d8-lint/pkg/config"
)

var (
	arrayPathRegex = regexp.MustCompile(`[\d+]`)
)

type EnumValidator struct {
	cfg      *config.OpenAPISettings
	key      string
	excludes map[string]struct{}
}

func NewEnumValidator(cfg *config.OpenAPISettings) EnumValidator {
	keyExcludes := make(map[string]struct{})

	for _, exc := range cfg.EnumFileExcludes["*"] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	return EnumValidator{
		cfg:      cfg,
		key:      "enum",
		excludes: keyExcludes,
	}
}

func (en EnumValidator) Run(moduleName, fileName, absoluteKey string, value any) error {
	for _, exc := range en.cfg.EnumFileExcludes[moduleName+":"+fileName] {
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

	err := en.validateEnumValues(absoluteKey, enum)

	return err
}

func (en EnumValidator) validateEnumValues(enumKey string, values []string) *multierror.Error {
	var res *multierror.Error
	for _, value := range values {
		err := en.validateEnumValue(value)
		if err != nil {
			res = multierror.Append(res, fmt.Errorf("enum '%s' is invalid: %w", enumKey, err))
		}
	}

	return res
}

func (EnumValidator) validateEnumValue(value string) error {
	if value == "" {
		return nil
	}

	vv := []rune(value)
	if (vv[0] < 'A' || vv[0] > 'Z') && (vv[0] < '0' || vv[0] > '9') {
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
