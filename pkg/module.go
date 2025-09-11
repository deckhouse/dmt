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

package pkg

import (
	"helm.sh/helm/v3/pkg/chart"

	"github.com/deckhouse/dmt/internal/storage"
)

//go:generate minimock -i github.com/deckhouse/dmt/pkg.Module -o ../internal/mocks/ -s "_mock.go"

// Module represents a DMT module interface that provides access to module metadata and resources.
// This interface is used by linters to access only the necessary module information,
// following the Interface Segregation Principle.
type Module interface {
	// GetName returns the name of the module
	GetName() string

	// GetNamespace returns the namespace where the module should be deployed
	GetNamespace() string

	// GetPath returns the filesystem path to the module directory
	GetPath() string

	// GetWerfFile returns the content of the werf.yaml file
	GetWerfFile() string

	// GetChart returns the Helm chart associated with the module
	GetChart() *chart.Chart

	// GetObjectStore returns the unstructured object store containing parsed Kubernetes resources
	GetObjectStore() *storage.UnstructuredObjectStore

	// GetStorage returns a map of all parsed Kubernetes resources indexed by ResourceIndex
	GetStorage() map[storage.ResourceIndex]storage.StoreObject
}
