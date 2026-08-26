package values

import (
	"os"
	"path/filepath"
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

	// Test override: vals is merged directly into the .Values tree (no wrapper),
	// so the original keys are preserved and the override keys are added at the
	// same level the renderer reads them from.
	values = &chartutil.Values{"foo": "bar"}
	vals := &chartutil.Values{"baz": "qux"}

	err = OverrideValues(values, vals)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, ok := (*values)["Values"]; ok {
		t.Errorf("did not expect a wrapping 'Values' key; vals must merge into the flat .Values tree")
	}

	if (*values)["foo"] != "bar" {
		t.Errorf("expected original key 'foo' to be preserved, got: %v", (*values)["foo"])
	}

	if (*values)["baz"] != "qux" {
		t.Errorf("expected override key 'baz' to be merged in, got: %v", (*values)["baz"])
	}
}

// Test for loadSchemaFromBytes
func TestLoadSchemaFromBytes(t *testing.T) {
	validYAML := []byte("type: object\nproperties:\n  foo:\n    type: string\n")

	schema, err := loadSchemaFromBytes(validYAML)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if schema == nil {
		t.Fatalf("expected schema, got nil")
	}

	invalidYAML := []byte("type: object\nproperties: [bad]")

	_, err = loadSchemaFromBytes(invalidYAML)
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
