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

	ch, err := loader.Load(path)
	if err != nil {
		return nil, err
	}
	reHelmModule := regexp.MustCompile(`{{ include "helm_lib_module_(?:image|common_image).* }}`)
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
