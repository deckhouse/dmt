package rules

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

type KeysRule struct {
	cfg *config.OpenAPISettings
	pkg.RuleMeta
}

func NewKeysRule(cfg *config.OpenAPISettings) *KeysRule {
	return &KeysRule{
		cfg: cfg,
		RuleMeta: pkg.RuleMeta{
			Name: "openapi-keys",
		},
	}
}

func (e *KeysRule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(e.GetName())

	haValidator := newKeyValidator(e.cfg)

	if err := openapi.Parse(haValidator.run, path); err != nil {
		errorList.WithFilePath(path).Errorf("openAPI file is not valid:\n%s", err)
	}
}

type keyValidator struct {
	bannedNames []string
}

func newKeyValidator(cfg *config.OpenAPISettings) keyValidator {
	return keyValidator{
		bannedNames: cfg.KeyBannedNames,
	}
}

func (kn keyValidator) run(absoluteKey string, value any) error {
	parts := strings.Split(absoluteKey, ".")
	if parts[len(parts)-1] != "enum" {
		return nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Map {
		m := make(map[any]any)
		for _, key := range rv.MapKeys() {
			v := rv.MapIndex(key)
			m[key.Interface()] = v.Interface()
		}

		err := checkMapForBannedKey(m, kn.bannedNames)
		if err != nil {
			return fmt.Errorf("validation error: wrong property: %w", err)
		}
	}

	return nil
}

func checkMapForBannedKey(m map[any]any, banned []string) error {
	for k, v := range m {
		for _, ban := range banned {
			if k == ban {
				return fmt.Errorf("%s is invalid name for property %s", ban, k)
			}
		}
		if nestedMap, ok := v.(map[any]any); ok {
			err := checkMapForBannedKey(nestedMap, banned)
			if err != nil {
				return err
			}
		}
		if nestedSlise, ok := v.([]any); ok {
			for _, item := range nestedSlise {
				if strKey, ok := item.(string); ok {
					for _, ban := range banned {
						if strKey == ban {
							return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
						}
					}
				}
				if nestedMap, ok := item.(map[any]any); ok {
					err := checkMapForBannedKey(nestedMap, banned)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
