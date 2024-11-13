package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/deckhouse/dmt/internal/storage"
)

const (
	ChartConfigFilename = "Chart.yaml"
)

type Module struct {
	name        string
	namespace   string
	path        string
	chart       *chart.Chart
	objectStore *storage.UnstructuredObjectStore
}

type ModuleList []*Module

func (m *Module) String() string {
	return fmt.Sprintf("{Name: %s, Namespace: %s, Path: %s}", m.name, m.namespace, m.path)
}

func (m *Module) GetName() string {
	if m == nil {
		return ""
	}

	return m.name
}

func (m *Module) GetNamespace() string {
	if m == nil {
		return ""
	}
	return m.namespace
}

func (m *Module) GetPath() string {
	if m == nil {
		return ""
	}
	return m.path
}

func (m *Module) GetChart() *chart.Chart {
	if m == nil {
		return nil
	}
	return m.chart
}

func (m *Module) GetMetadata() *chart.Metadata {
	if m.chart == nil || m.chart.Metadata == nil {
		return nil
	}

	return m.chart.Metadata
}

func (m *Module) GetObjectStore() *storage.UnstructuredObjectStore {
	if m == nil {
		return nil
	}
	return m.objectStore
}

func (m *Module) GetStorage() map[storage.ResourceIndex]storage.StoreObject {
	if m == nil || m.objectStore == nil {
		return nil
	}
	return m.objectStore.Storage
}

func NewModule(path string) (*Module, error) {
	name, err := getModuleName(path)
	if err != nil {
		return nil, err
	}
	namespace, err := getNamespace(path)
	if err != nil {
		return nil, err
	}
	module := &Module{
		name:      name,
		namespace: namespace,
		path:      path,
	}

	ch, err := loader.Load(path)
	if err != nil {
		return nil, err
	}

	module.chart = ch

	values, err := ComposeValuesFromSchemas(module)
	if err != nil {
		return nil, nil
	}
	objectStore := storage.NewUnstructuredObjectStore()
	err = RunRender(module, values, objectStore)
	if err != nil {
		return nil, nil
	}
	module.objectStore = objectStore

	return module, nil
}

func getModuleName(path string) (name string, err error) {
	yamlFile, err := os.ReadFile(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		return "", err
	}

	var ch struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &ch)
	if err != nil {
		return "", err
	}

	return ch.Name, nil
}

func getNamespace(path string) (name string, err error) {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(content), " \t\n"), nil
}
