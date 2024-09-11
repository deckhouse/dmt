package validators

import (
	"fmt"
	"reflect"
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
		if strKey, ok := k.(string); ok {
			for _, ban := range banned {
				if strKey == ban {
					return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
				}
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

func (knv KeyNameValidator) Run(file, _ string, value any) error {
	object, ok := value.(map[any]any)
	if !ok {
		fmt.Println("Possible Bug? Have to be a map", reflect.TypeOf(value))
		return nil
	}
	err := checkMapForBannedKey(object, bannedNames)
	if err != nil {
		return fmt.Errorf("%s file validation error: wrong property: %w", file, err)
	}
	return nil
}
