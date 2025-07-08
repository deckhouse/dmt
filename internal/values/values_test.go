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

package values

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/chartutil"
)

func TestOverrideValues(t *testing.T) {
	// Test nil vals
	values := &chartutil.Values{"foo": "bar"}
	err := OverrideValues(values, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if (*values)["foo"] != "bar" {
		t.Errorf("expected values to be unchanged when vals is nil")
	}

	// Test override
	values = &chartutil.Values{"foo": "bar"}
	vals := &chartutil.Values{"baz": "qux"}
	err = OverrideValues(values, vals)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if v, ok := (*values)["Values"]; !ok {
		t.Errorf("expected 'Values' key to be present after override")
	} else {
		valsMap, ok := v.(chartutil.Values)
		if !ok {
			t.Errorf("expected 'Values' to be of type chartutil.Values")
		}
		if !reflect.DeepEqual(valsMap, *vals) {
			t.Errorf("expected 'Values' to equal vals, got: %v", valsMap)
		}
	}
}

// Test for LoadSchemaFromBytes
func TestLoadSchemaFromBytes(t *testing.T) {
	validYAML := []byte("type: object\nproperties:\n  foo:\n    type: string\n")
	schema, err := LoadSchemaFromBytes(validYAML)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if schema == nil {
		t.Fatalf("expected schema, got nil")
	}

	invalidYAML := []byte("type: object\nproperties: [bad]")
	_, err = LoadSchemaFromBytes(invalidYAML)
	if err == nil {
		t.Errorf("expected error for invalid YAML, got nil")
	}
}

// Test for prepareSchemas
func TestPrepareSchemas(t *testing.T) {
	validYAML := []byte("type: object\nproperties:\n  foo:\n    type: string\n")
	schemas, err := prepareSchemas(validYAML, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if schemas[ConfigValuesSchema] == nil {
		t.Errorf("expected config schema to be present")
	}

	schemas, err = prepareSchemas(nil, validYAML)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if schemas[ValuesSchema] == nil {
		t.Errorf("expected values schema to be present")
	}
	if schemas[HelmValuesSchema] == nil {
		t.Errorf("expected helm values schema to be present")
	}

	schemas, err = prepareSchemas(validYAML, validYAML)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if schemas[ConfigValuesSchema] == nil || schemas[ValuesSchema] == nil || schemas[HelmValuesSchema] == nil {
		t.Errorf("expected all schemas to be present")
	}
}

// Test for GetGlobalValues
func TestGetGlobalValues(t *testing.T) {
	_, err := GetGlobalValues("")
	if err != nil {
		t.Errorf("expected no error for embedded, got: %v", err)
	}

	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "global-hooks", "openapi"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml"), []byte("type: object\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml"), []byte("type: object\n"), 0o600)
	_, err = GetGlobalValues(dir)
	if err != nil {
		t.Errorf("expected no error for valid files, got: %v", err)
	}

	dir2 := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir2, "global-hooks", "openapi"), 0o755)
	// не создаём ни одного файла
	_, err = GetGlobalValues(dir2)
	if err == nil {
		t.Errorf("expected error for missing files, got nil")
	}

	dir3 := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir3, "global-hooks", "openapi"), 0o755)
	_ = os.WriteFile(filepath.Join(dir3, "global-hooks", "openapi", "config-values.yaml"), []byte("type: object\n"), 0o600)
	_, err = GetGlobalValues(dir3)
	if err == nil {
		t.Errorf("expected error if one file is missing, got nil")
	}
}

// Test for readConfigFiles
func TestReadConfigFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "global-hooks", "openapi"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "global-hooks", "openapi", "config-values.yaml"), []byte("foo"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "global-hooks", "openapi", "values.yaml"), []byte("bar"), 0o600)
	cfg, vals, err := readConfigFiles(dir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(cfg) != "foo" || string(vals) != "bar" {
		t.Errorf("unexpected file contents: %s, %s", cfg, vals)
	}

	dir2 := t.TempDir()
	_, _, err = readConfigFiles(dir2)
	if err == nil {
		t.Errorf("expected error for missing config, got nil")
	}
}

// Test for GetModuleValues
func TestGetModuleValues(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "openapi"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "openapi", "config-values.yaml"), []byte("type: object\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "openapi", "values.yaml"), []byte("type: object\n"), 0o600)
	_, err := GetModuleValues(dir)
	if err != nil {
		t.Errorf("expected no error for valid files, got: %v", err)
	}

	dir2 := t.TempDir()
	_, err = GetModuleValues(dir2)
	if err == nil {
		t.Errorf("expected error for missing files, got nil")
	}
}
