/*
Copyright 2026 Flant JSC

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

package schemas

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

const sampleCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.deckhouse.io
spec:
  group: example.deckhouse.io
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [size]
              properties:
                size:
                  type: integer
                  minimum: 1
                mode:
                  type: string
                  enum: [fast, slow]
                extra:
                  type: object
                  x-kubernetes-preserve-unknown-fields: true
                value:
                  x-kubernetes-int-or-string: true
`

func loadStoreWithCRD(t *testing.T) *Store {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "widget.yaml"), []byte(sampleCRD), 0o600); err != nil {
		t.Fatal(err)
	}

	s := New()
	if err := s.LoadModuleCRDs(dir); err != nil {
		t.Fatalf("LoadModuleCRDs: %v", err)
	}

	return s
}

func mustObj(t *testing.T, manifest string) map[string]any {
	t.Helper()

	var obj map[string]any
	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	return obj
}

// nullKeywordCRD reproduces deckhouse CRDs (e.g. user-authn) that leave schema
// keywords empty, which YAML parses as null. maxLength/description as null are
// invalid against the JSON Schema metaschema and used to fail the whole CRD
// load; sanitizeMap must drop them so the schema still compiles.
const nullKeywordCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gadgets.example.deckhouse.io
spec:
  group: example.deckhouse.io
  names:
    kind: Gadget
    plural: gadgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                name:
                  type: string
                  maxLength:
                  description:
`

func TestLoadModuleCRDs_NullKeywordsDropped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gadget.yaml"), []byte(nullKeywordCRD), 0o600); err != nil {
		t.Fatal(err)
	}

	s := New()
	if err := s.LoadModuleCRDs(dir); err != nil {
		t.Fatalf("LoadModuleCRDs must tolerate null keyword values, got: %v", err)
	}

	// The dropped nulls must be surfaced as notes so the author can clean them up.
	notes := s.ModuleCRDNotes()
	gotPaths := make(map[string]bool, len(notes))
	for _, n := range notes {
		if n.Kind != "Gadget" || n.Group != "example.deckhouse.io" || n.Version != "v1" {
			t.Errorf("unexpected note identity: %+v", n)
		}
		gotPaths[n.Path] = true
	}

	for _, want := range []string{
		"properties/spec/properties/name/maxLength",
		"properties/spec/properties/name/description",
	} {
		if !gotPaths[want] {
			t.Errorf("expected a note for dropped null keyword %q; got notes %+v", want, notes)
		}
	}

	// The CRD must still be usable for validation after the nulls are dropped.
	res := s.Validate(mustObj(t, `
apiVersion: example.deckhouse.io/v1
kind: Gadget
metadata:
  name: g1
spec:
  name: hello
`))
	if !res.Found {
		t.Fatal("expected the compiled Gadget schema to be found")
	}

	if !res.Valid() {
		t.Fatalf("expected a valid object, got errors: %v", res.Errors)
	}
}

// repoCertificateCRD is a trimmed cert-manager Certificate CRD that, unlike the
// bundled catalog snapshot, declares the modern spec.certificateOwnerRef field.
const repoCertificateCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
spec:
  group: cert-manager.io
  names:
    kind: Certificate
    plural: certificates
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [secretName]
              properties:
                secretName:
                  type: string
                certificateOwnerRef:
                  type: boolean
`

// certWithOwnerRef renders a Certificate using the field missing from the
// bundled catalog. It is valid for the cert-manager version deckhouse ships.
const certWithOwnerRef = `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: hubble
  namespace: d8-cni-cilium
spec:
  secretName: hubble-tls
  certificateOwnerRef: true
`

// makeDeckhouseTree creates a minimal directory that DeckhouseRoot recognises as
// a repository root, with one module shipping the given CRD under crds/.
func makeDeckhouseTree(t *testing.T, module, crd string) string {
	t.Helper()

	root := t.TempDir()
	openapi := filepath.Join(root, "global-hooks", "openapi")
	if err := os.MkdirAll(openapi, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config-values.yaml", "values.yaml"} {
		if err := os.WriteFile(filepath.Join(openapi, f), []byte("type: object\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Nest the CRD in a subdirectory (crds/cert-manager/…) exactly as deckhouse
	// ships cert-manager's CRDs, so the loader must recurse to find it.
	crdsDir := filepath.Join(root, "modules", module, "crds", "cert-manager")
	if err := os.MkdirAll(crdsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crdsDir, "cert-manager.io_certificates.yaml"), []byte(crd), 0o600); err != nil {
		t.Fatal(err)
	}

	return root
}

// TestRepoCRDsOverrideCatalog proves the deckhouse repository's own CRDs win over
// the bundled catalog: a Certificate using certificateOwnerRef is rejected by the
// stale catalog but accepted once the repo overlay is in effect.
func TestRepoCRDsOverrideCatalog(t *testing.T) {
	root := makeDeckhouseTree(t, "101-cert-manager", repoCertificateCRD)

	if got := DeckhouseRoot(filepath.Join(root, "modules", "500-cilium-hubble")); got != root {
		t.Fatalf("DeckhouseRoot = %q, want %q", got, root)
	}

	obj := mustObj(t, certWithOwnerRef)

	// Control: against the bundled catalog alone the field is rejected.
	catalogOnly := New()
	if err := catalogOnly.Prepare([]map[string]any{obj}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if res := catalogOnly.Validate(obj); res.Found && res.Valid() {
		t.Fatal("expected the stale catalog schema to reject certificateOwnerRef (control)")
	}

	// With the repo overlay the same object validates against deckhouse's CRD.
	s := New()
	s.UseRepoCRDs(LoadRepoCRDs(root))
	if err := s.Prepare([]map[string]any{obj}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	res := s.Validate(obj)
	if res.SchemaSource != sourceDeckhouse {
		t.Fatalf("expected schema source %q, got %q", sourceDeckhouse, res.SchemaSource)
	}
	if !res.Valid() {
		t.Fatalf("expected repo CRD to accept certificateOwnerRef, got errors: %v", res.Errors)
	}
}

// TestValidate_NullOptionalFieldStripped reproduces manifests that render an
// optional string field as an explicit null (e.g. `caFile: null` when a chart
// leaves it empty). Kubernetes treats such nulls as unset, so validation must
// not flag "got null, want string".
func TestValidate_NullOptionalFieldStripped(t *testing.T) {
	s := loadStoreWithCRD(t)

	obj := mustObj(t, `
apiVersion: example.deckhouse.io/v1alpha1
kind: Widget
metadata:
  name: w1
spec:
  size: 3
  mode: null
`)

	res := s.Validate(obj)
	if !res.Found {
		t.Fatal("expected schema to be found for Widget")
	}

	if !res.Valid() {
		t.Fatalf("expected null optional field to be treated as unset, got errors: %v", res.Errors)
	}
}

// TestValidate_NullRequiredFieldStillFails guards the flip side: stripping a null
// must not mask a genuinely missing required field.
func TestValidate_NullRequiredFieldStillFails(t *testing.T) {
	s := loadStoreWithCRD(t)

	obj := mustObj(t, `
apiVersion: example.deckhouse.io/v1alpha1
kind: Widget
metadata:
  name: w1
spec:
  size: null
`)

	res := s.Validate(obj)
	if !res.Found {
		t.Fatal("expected schema to be found for Widget")
	}

	if res.Valid() {
		t.Fatal("expected a null required field to be reported as missing")
	}
}

func TestValidateModuleCRD_Valid(t *testing.T) {
	s := loadStoreWithCRD(t)

	obj := mustObj(t, `
apiVersion: example.deckhouse.io/v1alpha1
kind: Widget
metadata:
  name: w1
spec:
  size: 3
  mode: fast
  extra:
    anything: goes
  value: "42"
`)

	res := s.Validate(obj)
	if !res.Found {
		t.Fatal("expected schema to be found for Widget")
	}

	if !res.Valid() {
		t.Fatalf("expected valid object, got errors: %v", res.Errors)
	}

	if res.SchemaSource != "module" {
		t.Fatalf("expected module source, got %q", res.SchemaSource)
	}
}

func TestValidateModuleCRD_Invalid(t *testing.T) {
	s := loadStoreWithCRD(t)

	// size below minimum, mode not in enum, required size missing handled elsewhere
	obj := mustObj(t, `
apiVersion: example.deckhouse.io/v1alpha1
kind: Widget
metadata:
  name: w1
spec:
  size: 0
  mode: turbo
`)

	res := s.Validate(obj)
	if !res.Found {
		t.Fatal("expected schema to be found")
	}

	if res.Valid() {
		t.Fatal("expected validation errors, got none")
	}

	if len(res.Errors) == 0 {
		t.Fatal("expected non-empty error list")
	}

	t.Logf("validation errors: %v", res.Errors)
}

func TestValidateModuleCRD_RequiredMissing(t *testing.T) {
	s := loadStoreWithCRD(t)

	obj := mustObj(t, `
apiVersion: example.deckhouse.io/v1alpha1
kind: Widget
metadata:
  name: w1
spec:
  mode: fast
`)

	res := s.Validate(obj)
	if res.Valid() {
		t.Fatal("expected error for missing required spec.size")
	}
}

func TestValidate_UnknownKindSkipped(t *testing.T) {
	s := New()

	obj := mustObj(t, `
apiVersion: totally.unknown.io/v1
kind: Nonexistent
metadata:
  name: x
`)

	res := s.Validate(obj)
	if res.Found {
		t.Fatal("expected no schema for unknown kind")
	}
}

func TestValidateEmbeddedK8sType(t *testing.T) {
	s := New()

	good := mustObj(t, `
apiVersion: v1
kind: Service
metadata:
  name: svc
spec:
  ports:
    - port: 80
      targetPort: 8080
`)

	bad := mustObj(t, `
apiVersion: v1
kind: Service
metadata:
  name: svc
spec:
  ports:
    - port: "not-a-number"
`)

	if err := s.Prepare([]map[string]any{good, bad}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	res := s.Validate(good)
	if !res.Found {
		t.Skip("embedded catalog has no Service schema; run scripts/gen-schemas.sh")
	}

	if res.SchemaSource != sourceK8s {
		t.Fatalf("expected k8s source, got %q", res.SchemaSource)
	}

	if !res.Valid() {
		t.Fatalf("valid Service reported errors: %v", res.Errors)
	}

	if r := s.Validate(bad); r.Valid() {
		t.Fatal("expected errors for Service with string port")
	}
}

func TestValidateEmbeddedCRD(t *testing.T) {
	s := New()

	// cert-manager Certificate is present in the datree catalog.
	obj := mustObj(t, `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: c
spec:
  secretName: s
  dnsNames: 12345
`)

	if err := s.Prepare([]map[string]any{obj}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	res := s.Validate(obj)
	if !res.Found {
		t.Skip("embedded catalog has no Certificate schema; run scripts/gen-schemas.sh")
	}

	if res.SchemaSource != sourceCRD {
		t.Fatalf("expected crd source, got %q", res.SchemaSource)
	}

	// dnsNames must be an array of strings, not a number.
	if res.Valid() {
		t.Fatal("expected type error for spec.dnsNames")
	}
}

func TestSplitAPIVersion(t *testing.T) {
	cases := map[string][2]string{
		"v1":                 {"", "v1"},
		"apps/v1":            {"apps", "v1"},
		"cert-manager.io/v1": {"cert-manager.io", "v1"},
	}

	for in, want := range cases {
		g, v := splitAPIVersion(in)
		if g != want[0] || v != want[1] {
			t.Errorf("splitAPIVersion(%q) = (%q,%q), want (%q,%q)", in, g, v, want[0], want[1])
		}
	}
}

func TestK8sLookupKeyShortGroup(t *testing.T) {
	got := k8sLookupKey("networking.k8s.io", "v1", "NetworkPolicy")
	want := "networkpolicy__networking__v1"

	if got != want {
		t.Errorf("k8sLookupKey = %q, want %q", got, want)
	}
}
