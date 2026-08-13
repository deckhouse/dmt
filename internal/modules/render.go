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

package modules

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/modules/render"
	"github.com/deckhouse/dmt/internal/modules/values"
	"github.com/deckhouse/dmt/internal/storage"
	dmtErrors "github.com/deckhouse/dmt/pkg/errors"
)

// RunRender strictly renders the module's chart through nelm's public
// action.ChartRender and stores the resulting objects for the linters. Strict
// rendering mirrors how deckhouse-controller installs the module: a template error
// fails the whole render, so the module is reported rather than half-linted. Image
// references resolve from dmt's value stubs (global.modulesImages, scanned from
// images/).
func RunRender(m *Module, vals chartutil.Values, objectStore *storage.UnstructuredObjectStore, errorList *dmtErrors.LintRuleErrorsList) error {
	objects, err := render.Render(context.Background(), m.GetNamespace(), m.GetName(), render.Options{
		Path:             m.GetPath(),
		Values:           vals,
		ExtraAPIVersions: render.ExtraAPIVersions(),
	})
	if err != nil {
		return fmt.Errorf("helm chart render: %w", err)
	}

	var resultErr error

	for _, obj := range objects {
		restoreStrippedNamespace(obj, m.GetNamespace())

		absPath, err := filepath.Abs(filepath.Join(m.GetPath(), obj.FilePath))
		if err != nil {
			absPath = obj.FilePath
		}

		docBytes, err := yaml.Marshal(obj.Object)
		if err != nil {
			resultErr = errors.Join(resultErr, err)

			continue
		}

		if err := objectStore.Put(absPath, obj.FilePath, obj.Object, docBytes); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}

	if resultErr != nil {
		errorList.WithFilePath(m.GetPath()).WithModule(m.GetName()).
			WithValue(resultErr.Error()).Error("module contains duplicate objects")
	}

	return nil
}

// clusterScopedKinds are the Kubernetes kinds that have no namespace. A rendered
// object of any other kind is namespaced; see restoreStrippedNamespace. An unknown
// custom-resource kind is treated as namespaced — the cross-object linters only key
// namespaced built-ins (ServiceAccount, Role, RoleBinding, …) by namespace, so a
// stray namespace on an unrecognised cluster-scoped CR is harmless.
var clusterScopedKinds = map[string]struct{}{
	"Namespace": {}, "Node": {}, "PersistentVolume": {}, "ComponentStatus": {},
	"ClusterRole": {}, "ClusterRoleBinding": {}, "CustomResourceDefinition": {},
	"ValidatingWebhookConfiguration": {}, "MutatingWebhookConfiguration": {},
	"ValidatingAdmissionPolicy": {}, "ValidatingAdmissionPolicyBinding": {},
	"StorageClass": {}, "VolumeAttachment": {}, "CSIDriver": {}, "CSINode": {},
	"PriorityClass": {}, "CertificateSigningRequest": {}, "APIService": {},
	"FlowSchema": {}, "PriorityLevelConfiguration": {}, "IngressClass": {},
	"RuntimeClass": {}, "ClusterIssuer": {},
}

// restoreStrippedNamespace re-attaches the module's namespace to a namespaced object
// that nelm rendered without one. nelm drops metadata.namespace whenever it equals
// the release namespace (as a real install does — the namespace is then implied), but
// the linters resolve objects across the store by namespace: an RBAC binding looks up
// its ServiceAccount by (name, kind, namespace), the placement rule checks a Role's
// namespace, and so on. Restoring the namespace here mirrors both the previous
// renderer and where the object actually lands in the cluster. Cluster-scoped objects
// legitimately have no namespace and are left untouched.
func restoreStrippedNamespace(obj render.Object, namespace string) {
	if namespace == "" || obj.GetNamespace() != "" {
		return
	}

	if _, clusterScoped := clusterScopedKinds[obj.GetKind()]; clusterScoped {
		return
	}

	obj.SetNamespace(namespace)
}

// RenderModuleWithValues renders the module at modulePath with the supplied user
// values (the ".Values" tree, e.g. holding "global" and the module's own section)
// and returns the rendered manifests keyed by chart-relative source file path.
//
// Image references resolve from dmt's image/registry stubs (digests scanned from
// images/, injected by values.HelmFormatModuleImages). Suitable for golden-snapshot tests.
func RenderModuleWithValues(modulePath string, userValues map[string]any) (map[string]string, error) {
	mod, err := newModuleFromPath(modulePath)
	if err != nil {
		return nil, err
	}

	if userValues == nil {
		userValues = map[string]any{}
	}

	renderValues, err := values.HelmFormatModuleImages(mod.GetPath(), mod.GetName(), userValues)
	if err != nil {
		return nil, fmt.Errorf("prepare render values: %w", err)
	}

	return renderModuleFiles(mod, renderValues)
}

// RenderModuleForValuesFile renders the module at modulePath using values
// auto-generated from its openapi schemas (config-values.yaml plus the given
// values file, e.g. "values.yaml" or "values_ce.yaml") combined with the global
// schema, mirroring the value generation dmt uses while linting. It returns the
// rendered manifests keyed by chart-relative source file path.
func RenderModuleForValuesFile(modulePath string, globalSchema *spec.Schema, valuesFile string) (map[string]string, error) {
	mod, err := newModuleFromPath(modulePath)
	if err != nil {
		return nil, err
	}

	renderValues, err := values.ComposeValuesFromSchemasForValuesFile(mod.GetPath(), mod.GetName(), globalSchema, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("compose values: %w", err)
	}

	return renderModuleFiles(mod, renderValues)
}

// renderModuleFiles strictly renders the module through nelm and returns the
// manifests keyed by chart-relative source path. Multiple documents rendered from
// one template file are joined with a "---" separator, preserving the previous
// map[path]->manifests shape its callers expect.
func renderModuleFiles(mod *Module, vals map[string]any) (map[string]string, error) {
	objects, err := render.Render(context.Background(), mod.GetNamespace(), mod.GetName(), render.Options{
		Path:             mod.GetPath(),
		Values:           vals,
		ExtraAPIVersions: render.ExtraAPIVersions(),
	})
	if err != nil {
		return nil, fmt.Errorf("render module: %w", err)
	}

	files := make(map[string]string, len(objects))

	for _, obj := range objects {
		data, err := yaml.Marshal(obj.Object)
		if err != nil {
			return nil, fmt.Errorf("marshal manifest %q: %w", obj.FilePath, err)
		}

		if existing := files[obj.FilePath]; existing != "" {
			files[obj.FilePath] = existing + "---\n" + string(data)

			continue
		}

		files[obj.FilePath] = string(data)
	}

	return files, nil
}
