package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
)

const (
	ChartConfigFilename = "Chart.yaml"
)

type Module struct {
	Name      string
	Namespace string
	Path      string
	Chart     *chart.Chart
}

type ModuleList []*Module

func (m *Module) String() string {
	return fmt.Sprintf("{Name: %s, Namespace: %s, Path: %s}", m.Name, m.Namespace, m.Path)
}

func (m *Module) GetName() string {
	return m.Name
}

func (m *Module) GetPath() string {
	return m.Path
}

func NewModule(path string) *Module {
	module := &Module{
		Name:      getModuleName(path),
		Namespace: getNamespace(path),
		Path:      path,
	}

	return module
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