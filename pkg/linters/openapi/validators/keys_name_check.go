package validators

import (
	"fmt"
	"reflect"

	"github.com/deckhouse/d8-lint/pkg/logger"
)

var (
	bannedNames = []string{"x-examples", "examples", "example"}
)

type KeyNameValidator struct {
}

func NewKeyNameValidator() KeyNameValidator {
	return KeyNameValidator{}
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

func (KeyNameValidator) Run(file, _ string, value any) error {
	m := make(map[any]any)
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map {
		logger.ErrorF("Possible Bug? Have to be a map. Type: %s, Value: %s, File: %s", reflect.TypeOf(value), value, file)
		return fmt.Errorf("not map")
	}
	for _, key := range rv.MapKeys() {
		v := rv.MapIndex(key)
		m[key.Interface()] = v.Interface()
	}

	err := checkMapForBannedKey(m, bannedNames)
	if err != nil {
		return fmt.Errorf("%s file validation error: wrong property: %w", file, err)
	}
	return nil
}
