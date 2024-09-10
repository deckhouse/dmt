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

func checkMapForBannedKey(m map[interface{}]interface{}, banned []string) error {
	for k, v := range m {
		if strKey, ok := k.(string); ok {
			for _, ban := range banned {
				if strKey == ban {
					return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
				}
			}
		}
		if nestedMap, ok := v.(map[interface{}]interface{}); ok {
			err := checkMapForBannedKey(nestedMap, banned)
			if err != nil {
				return err
			}
		}
		if nestedSlise, ok := v.([]interface{}); ok {
			for _, item := range nestedSlise {
				if strKey, ok := item.(string); ok {
					for _, ban := range banned {
						if strKey == ban {
							return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
						}
					}
				}
				if nestedMap, ok := item.(map[interface{}]interface{}); ok {
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

func (knv KeyNameValidator) Run(file, _ string, value interface{}) error {
	object, ok := value.(map[interface{}]interface{})
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
