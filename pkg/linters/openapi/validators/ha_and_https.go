package validators

import (
	"fmt"
	"reflect"

	"github.com/deckhouse/d8-lint/pkg/logger"
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

	for key := range m {
		if key == "default" {
			if absoluteKeysExcludes[file] == absoluteKey {
				continue
			}
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
