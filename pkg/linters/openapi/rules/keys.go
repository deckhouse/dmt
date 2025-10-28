/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rules

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/deckhouse/dmt/internal/openapi"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

type KeysRule struct {
	cfg *pkg.OpenAPILinterConfig
	pkg.RuleMeta
	rootPath string
}

func NewKeysRule(cfg *pkg.OpenAPILinterConfig, rootPath string) *KeysRule {
	return &KeysRule{
		cfg: cfg,
		RuleMeta: pkg.RuleMeta{
			Name: "keys",
		},
		rootPath: rootPath,
	}
}

func (e *KeysRule) Run(path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(e.GetName())

	shortPath, _ := filepath.Rel(e.rootPath, path)
	haValidator := newKeyValidator(e.cfg)

	if err := openapi.Parse(haValidator.run, path); err != nil {
		errorList.WithFilePath(shortPath).Errorf("openAPI file is not valid:\n%s", err)
	}
}

type keyValidator struct {
	bannedNames []string
}

func newKeyValidator(cfg *pkg.OpenAPILinterConfig) keyValidator {
	return keyValidator{
		bannedNames: cfg.ExcludeRules.KeyBannedNames,
	}
}

func (kn keyValidator) run(absoluteKey string, value any) error {
	parts := strings.Split(absoluteKey, ".")
	if parts[len(parts)-1] != "enum" {
		return nil
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice:
		for i := range rv.Len() {
			item := rv.Index(i).Interface()
			if strKey, ok := item.(string); ok {
				for _, ban := range kn.bannedNames {
					if strKey == ban {
						return fmt.Errorf("%s is invalid name for property %s", ban, strKey)
					}
				}
			}
		}
	case reflect.Map:
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
