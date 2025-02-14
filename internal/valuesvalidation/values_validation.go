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

package valuesvalidation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/logger"
)

type ValuesValidator struct {
	GlobalSchemaStorage  *validation.SchemaStorage
	ModuleSchemaStorages map[string]*validation.SchemaStorage
}

//go:embed global-openapi/config-values.yaml
var globalConfigBytes []byte

//go:embed global-openapi/values.yaml
var globalValuesBytes []byte

func NewValuesValidator(moduleName, modulePath string) (*ValuesValidator, error) {
	globalSchemaStorage, err := validation.NewSchemaStorage(globalConfigBytes, globalValuesBytes)
	if err != nil {
		return nil, fmt.Errorf("parse global openAPI schemas: %w", err)
	}

	if moduleName == "" || modulePath == "" {
		return &ValuesValidator{GlobalSchemaStorage: globalSchemaStorage}, nil
	}

	openAPIPath := filepath.Join(modulePath, "openapi")
	configBytes, valuesBytes, err := utils.ReadOpenAPIFiles(openAPIPath)
	if err != nil {
		return nil, fmt.Errorf("module '%s' read openAPI schemas: %w", moduleName, err)
	}

	moduleSchemaStorage, err := validation.NewSchemaStorage(configBytes, valuesBytes)
	if err != nil {
		return nil, fmt.Errorf("parse module openAPI schemas: %w", err)
	}

	return &ValuesValidator{
		GlobalSchemaStorage: globalSchemaStorage,
		ModuleSchemaStorages: map[string]*validation.SchemaStorage{
			moduleName: moduleSchemaStorage,
		},
	}, nil
}

// ValidateValues is an adapter between JSONRepr and Values
func (vv *ValuesValidator) ValidateValues(moduleName string, values chartutil.Values) error {
	obj := values["Values"].(map[string]any)

	err := vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = vv.ValidateModuleValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateHelmValues(moduleName, values string) error {
	var obj map[string]any
	err := yaml.Unmarshal([]byte(values), &obj)
	if err != nil {
		return err
	}

	err = vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)
	err = vv.ValidateModuleHelmValues(valuesKey, obj)
	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateJSONValues(moduleName string, values []byte, configValues bool) error {
	obj := make(map[string]any)
	err := json.Unmarshal(values, &obj)
	if err != nil {
		return err
	}

	err = vv.ValidateGlobalValues(obj)
	if err != nil {
		return err
	}

	valuesKey := utils.ModuleNameToValuesKey(moduleName)

	if configValues {
		err = vv.ValidateConfigValues("config", obj)
	} else {
		err = vv.ValidateModuleValues(valuesKey, obj)
	}

	if err != nil {
		return err
	}
	return nil
}

func (vv *ValuesValidator) ValidateGlobalValues(obj utils.Values) error {
	return vv.GlobalSchemaStorage.ValidateValues(utils.GlobalValuesKey, obj)
}

func (vv *ValuesValidator) ValidateModuleValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		logger.WarnF("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateValues(moduleName, obj)
}

func (vv *ValuesValidator) ValidateModuleHelmValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		logger.WarnF("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateModuleHelmValues(moduleName, obj)
}

func (vv *ValuesValidator) ValidateConfigValues(moduleName string, obj utils.Values) error {
	ss := vv.ModuleSchemaStorages[moduleName]
	if ss == nil {
		logger.WarnF("schema storage for '%s' is not found", moduleName)
		return nil
	}

	return vv.ModuleSchemaStorages[moduleName].ValidateConfigValues(moduleName, obj)
}
