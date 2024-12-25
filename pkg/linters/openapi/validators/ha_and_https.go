package validators

import (
	"fmt"
	"reflect"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/pkg/config"
)

type HAValidator struct {
	absoluteKeysExcludes map[string]string
}

func NewHAValidator(cfg *config.OpenAPISettings) HAValidator {
	return HAValidator{
		absoluteKeysExcludes: cfg.HAAbsoluteKeysExcludes,
	}
}

func (ha HAValidator) Run(moduleName, file, absoluteKey string, value any) error {
	// Ignore key inside a deep structure, like properties.internal.spec.xxx
	if absoluteKey != "properties.highAvailability" {
		return nil
	}

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
			if ha.absoluteKeysExcludes[moduleName+":"+file] == absoluteKey {
				continue
			}
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
