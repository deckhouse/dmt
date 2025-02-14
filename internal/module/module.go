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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"

	"github.com/deckhouse/dmt/internal/storage"
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

	ch, err := LoadModuleAsChart(name, path)
	if err != nil {
		return nil, err
	}

	reHelmModule := regexp.MustCompile(`{{ include "helm_lib_module_(?:image|common_image|init).* }}`)
	reImageDigest := regexp.MustCompile(`\$\.Values\.global\.modulesImages\.digests\.\S*`)
	for i := range ch.Templates {
		var outputLines strings.Builder
		scanner := bufio.NewScanner(bytes.NewReader(ch.Templates[i].Data))
		for scanner.Scan() {
			line := scanner.Text()
			if pos := strings.Index(line, `:= include "helm_lib_module_`); pos > -1 {
				line = line[:pos] + `:= "imageHash-` + name + `-container" }}`
			}
			if pos := strings.Index(line, `:= (include "helm_lib_module_`); pos > -1 {
				line = line[:pos] + `:= "example.domain.com:tags"  | splitn ":" 2 }}`
			}
			if pos := strings.Index(line, "image: "); pos > -1 {
				line = line[:pos] + "image: registry.example.com/deckhouse@imageHash-" + name + "-container"
			}
			line = reHelmModule.ReplaceAllString(line, "imageHash-"+name+"-container")
			line = reImageDigest.ReplaceAllString(line, "$.Values.global.modulesImages.digests.common")
			outputLines.WriteString(line + "\n")
		}
		ch.Templates[i].Data = []byte(outputLines.String())
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

func getModuleName(path string) (string, error) {
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

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}
