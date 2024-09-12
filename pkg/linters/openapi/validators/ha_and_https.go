package validators

import (
	"fmt"
	"reflect"
)

var (
	absoluteKeysExcludes = map[string]string{
		"modules/150-user-authn/openapi/config-values.yaml": "properties.publishAPI.properties.https",
		"global-hooks/openapi/config-values.yaml":           "properties.modules.properties.https",
	}
)

type HAValidator struct {
}

func NewHAValidator() HAValidator {
	return HAValidator{}
}

func (HAValidator) Run(file, absoluteKey string, value any) error {
	values, ok := value.(map[any]any)
	if !ok {
		fmt.Printf("Possible Bug? Have to be a map. Type: %s, Value: %s, File: %s, Key: %s\n", reflect.TypeOf(value), value, file, absoluteKey)
		return nil
	}

	for key := range values {
		if key == "default" {
			if absoluteKeysExcludes[file] == absoluteKey {
				continue
			}
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
