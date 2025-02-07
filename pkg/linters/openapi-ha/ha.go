package openapiha

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/deckhouse/dmt/pkg/config"
)

type HAValidator struct {
	cfg *config.OpenAPIHASettings
}

func NewHAValidator(cfg *config.OpenAPIHASettings) HAValidator {
	return HAValidator{
		cfg: cfg,
	}
}

func (HAValidator) Run(_, absoluteKey string, value any) error {
	// Ignore key inside a deep structure, like properties.internal.spec.xxx
	if absoluteKey != "properties.highAvailability" {
		return nil
	}

	parts := strings.Split(absoluteKey, ".")
	key := parts[len(parts)-1]
	if key != "highAvailability" && key != "https" {
		return nil
	}

	m := make(map[any]any)
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Map {
		return fmt.Errorf("possible Bug? Have to be a map. Type: %s, Value: %s", reflect.TypeOf(value), value)
	}

	for _, key := range rv.MapKeys() {
		v := rv.MapIndex(key)
		m[key.Interface()] = v.Interface()
	}

	for key := range m {
		if key == "default" {
			return fmt.Errorf("%s is invalid: must have no default value", absoluteKey)
		}
	}

	return nil
}
