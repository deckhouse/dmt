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
	ChartConfigFilename  = "Chart.yaml"
	ModuleConfigFilename = "module.yaml"
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

	module := &Module{
		name:      name,
		namespace: getNamespace(path),
		path:      path,
	}

	err = checkHelmChart(name, path)
	if err != nil {
		return nil, err
	}

	ch, err := loader.Load(path)
	if err != nil {
		return nil, err
	}

	module.chart = ch

	values, err := ComposeValuesFromSchemas(module)
	if err != nil {
		return nil, err
	}
	objectStore := storage.NewUnstructuredObjectStore()
	err = RunRender(module, values, objectStore)
	if err != nil {
		return nil, err
	}
	module.objectStore = objectStore

	return module, nil
}

func getModuleName(path string) (name string, err error) {
	stat, err := os.Stat(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		stat, err = os.Stat(filepath.Join(path, ModuleConfigFilename))
		if err != nil {
			return "", err
		}
	}
	yamlFile, err := os.ReadFile(filepath.Join(path, stat.Name()))
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

func getNamespace(path string) (name string) {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}

// isHelmChart check, could it be considered as helm chart or not
func checkHelmChart(name, path string) error {
	chartPath := filepath.Join(path, "Chart.yaml")

	_, err := os.Stat(chartPath)
	if err == nil {
		// Chart.yaml exists, consider this module as helm chart
		return nil
	}

	if os.IsNotExist(err) {
		// Chart.yaml does not exist
		return createChartYaml(name, chartPath)
	}

	return err
}

func createChartYaml(name, chartPath string) error {
	// we already have versions like 0.1.0 or 0.1.1
	// to keep helm updatable, we have to increment this version
	// new minor version of addon-operator seems reasonable to increase minor version of a helm chart
	data := fmt.Sprintf(`name: %s
version: 0.2.0`, name)

	return os.WriteFile(chartPath, []byte(data), 0o600)
}
