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

package module

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/spec"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/internal/values"
	"github.com/deckhouse/dmt/internal/werf"
	"github.com/deckhouse/dmt/pkg/config"
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
	werfFile    string

	linterConfig *config.ModuleConfig
}

type ModuleList []*Module

type ModuleYaml struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type ChartYaml struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

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

func (m *Module) GetWerfFile() string {
	if m == nil {
		return ""
	}
	return m.werfFile
}

func (m *Module) GetModuleConfig() *config.ModuleConfig {
	if m == nil {
		return nil
	}
	return m.linterConfig
}

func (m *Module) MergeRootConfig(cfg *config.RootConfig) {
	m.linterConfig.LintersSettings.MergeGlobal(&cfg.GlobalSettings.Linters)
}

func NewModule(path string, vals *chartutil.Values, globalSchema *spec.Schema) (*Module, error) {
	module, err := newModuleFromPath(path)
	if err != nil {
		return nil, err
	}

	schemas, err := ComposeValuesFromSchemas(module, globalSchema)
	if err != nil {
		return nil, err
	}

	if err = values.OverrideValues(&schemas, vals); err != nil {
		return nil, fmt.Errorf("failed to override values from file: %w", err)
	}

	objectStore := storage.NewUnstructuredObjectStore()
	err = RunRender(module, schemas, objectStore)
	if err != nil {
		return nil, err
	}
	module.objectStore = objectStore

	werfFile, err := werf.GetWerfConfig(path)
	if err == nil && werfFile != "" {
		module.werfFile = werfFile
	}

	cfg := &config.ModuleConfig{}
	if err := config.NewLoader(cfg, path).Load(); err != nil {
		return nil, fmt.Errorf("can not parse module config: %w", err)
	}

	module.linterConfig = cfg

	return module, nil
}

func remapChart(ch *chart.Chart) {
	remapTemplates(ch)
	for _, dependency := range ch.Dependencies() {
		remapChart(dependency)
	}
}

//go:embed templates/_module_name.tpl
var moduleNameTemplate []byte

//go:embed templates/_module_image.tpl
var moduleImageTemplate []byte

func remapTemplates(ch *chart.Chart) {
	for _, template := range ch.Templates {
		switch template.Name {
		case "templates/_module_name.tpl":
			template.Data = moduleNameTemplate
		case "templates/_module_image.tpl":
			template.Data = moduleImageTemplate
		}
	}
}

func newModuleFromPath(path string) (*Module, error) {
	moduleYamlConfig, err := ParseModuleConfigFile(path)
	if err != nil {
		return nil, err
	}
	chartYamlConfig, err := ParseChartFile(path)
	if err != nil {
		return nil, err
	}

	var info ModuleYaml
	info.Name = GetModuleName(moduleYamlConfig, chartYamlConfig)
	if moduleYamlConfig != nil && moduleYamlConfig.Namespace != "" {
		info.Namespace = moduleYamlConfig.Namespace
	}

	if info.Namespace == "" {
		// fallback to the 'test' .namespace file
		namespace := getNamespace(path)
		if namespace != "" {
			return nil, fmt.Errorf("module %q has no namespace", info.Name)
		}
		info.Namespace = namespace
	}

	moduleChart, err := LoadModuleAsChart(info.Name, path)
	if err != nil {
		return nil, err
	}
	remapChart(moduleChart)

	resultModule := &Module{
		name:      info.Name,
		namespace: info.Namespace,
		path:      path,
		chart:     moduleChart,
	}

	return resultModule, nil
}

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}

func ParseModuleConfigFile(path string) (*ModuleYaml, error) {
	moduleFilename := filepath.Join(path, ModuleConfigFilename)
	yamlFile, err := fsutils.ReadFile(moduleFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var deckhouseModule ModuleYaml
	err = yaml.Unmarshal(yamlFile, &deckhouseModule)
	if err != nil {
		return nil, err
	}

	return &deckhouseModule, nil
}

func ParseChartFile(path string) (*ChartYaml, error) {
	chartFilename := filepath.Join(path, ChartConfigFilename)
	yamlFile, err := fsutils.ReadFile(chartFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var chartYaml ChartYaml
	err = yaml.Unmarshal(yamlFile, &chartYaml)
	if err != nil {
		return nil, err
	}

	return &chartYaml, nil
}

func GetModuleName(moduleYamlFile *ModuleYaml, chartYamlFile *ChartYaml) string {
	if moduleYamlFile != nil && moduleYamlFile.Name != "" {
		return moduleYamlFile.Name
	}
	if chartYamlFile != nil && chartYamlFile.Name != "" {
		return chartYamlFile.Name
	}
	return ""
}
