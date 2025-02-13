package openapi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/deckhouse/dmt/pkg/config"
)

type KeyValidator struct {
	bannedNames []string
}

func NewKeyValidator(cfg *config.OpenAPISettings) KeyValidator {
	return KeyValidator{
		bannedNames: cfg.KeyBannedNames,
	}
}

func (kn KeyValidator) Run(_, absoluteKey string, value any) error {
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
