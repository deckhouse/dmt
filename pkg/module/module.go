package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const (
	ChartConfigFilename = "Chart.yaml"
)

type Module struct {
	name      string
	namespace string
	path      string
	chart     *chart.Chart
}

type ModuleList []*Module

func (m *Module) String() string {
	return fmt.Sprintf("{Name: %s, Namespace: %s, Path: %s}", m.name, m.namespace, m.path)
}

func (m *Module) GetName() string {
	return m.name
}

func (m *Module) GetNamespace() string {
	return m.namespace
}

func (m *Module) GetPath() string {
	return m.path
}

func (m *Module) GetChart() *chart.Chart {
	return m.chart
}

func (m *Module) GetMetadata() *chart.Metadata {
	if m.chart == nil {
		return nil
	}
	if m.chart.Metadata == nil {
		m.chart.Metadata = &chart.Metadata{}
	}
	return m.chart.Metadata
}

func NewModule(path string) (*Module, error) {
	ch, err := loader.Load(path)

	module := &Module{
		name:      getModuleName(path),
		namespace: getNamespace(path),
		path:      path,
		chart:     ch,
	}

	return module, err
}

func getModuleName(path string) string {
	yamlFile, err := os.ReadFile(filepath.Join(path, ChartConfigFilename))
	if err != nil {
		return ""
	}

	var ch struct {
		Name string `yaml:"name"`
	}
	err = yaml.Unmarshal(yamlFile, &ch)
	if err != nil {
		return ""
	}

	return ch.Name
}

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}
