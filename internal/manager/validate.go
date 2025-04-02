package manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/module"
)

type moduleYaml struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type chartYaml struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func (m *Manager) validateModule(path string) error {
	var errs error
	m.errors = m.errors.WithLinterID("module").WithRule("definition-file")
	// validate module.yaml and Chart.yaml
	chartYamlFile, err := parseChartFile(path)
	if err != nil {
		err = fmt.Errorf("failed to parse Chart.yaml: %w", err)
		errs = errors.Join(errs, err)
		m.errors.Error(err.Error())
	}
	moduleYamlFile, err := parseModuleConfigFile(path)
	if err != nil {
		err = fmt.Errorf("failed to parse module.yaml: %w", err)
		errs = errors.Join(errs, err)
		m.errors.Error(err.Error())
	}
	if chartYamlFile != nil {
		if chartYamlFile.Name == "" {
			err := fmt.Errorf("property `name` in Chart.yaml is empty")
			errs = errors.Join(errs, err)
			m.errors.Error(err.Error())
		}
		if chartYamlFile.Version == "" {
			err := fmt.Errorf("property `version` in Chart.yaml is empty")
			errs = errors.Join(errs, err)
			m.errors.Error(err.Error())
		}
	}
	if moduleYamlFile != nil {
		if moduleYamlFile.Name == "" {
			err := fmt.Errorf("module.yaml `name` is empty")
			errs = errors.Join(errs, err)
			m.errors.Error(err.Error())
		}
		if moduleYamlFile.Namespace == "" {
			err := fmt.Errorf("module.yaml `namespace` is empty")
			errs = errors.Join(errs, err)
			m.errors.Error(err.Error())
		}
	}
	if moduleYamlFile != nil && chartYamlFile != nil {
		if chartYamlFile.Name != "" && moduleYamlFile.Name != "" && chartYamlFile.Name != moduleYamlFile.Name {
			err := fmt.Errorf("module.yaml name (%s) does not match Chart.yaml name (%s)", moduleYamlFile.Name, chartYamlFile.Name)
			errs = errors.Join(errs, err)
			m.errors.Errorf(err.Error())
		}
	}

	// validate namespace
	if moduleYamlFile == nil && chartYamlFile != nil {
		if getNamespace(path) == "" {
			err := fmt.Errorf("file Chart.yaml is present, but .namespace file is missing")
			errs = errors.Join(errs, err)
			m.errors.Errorf(err.Error())
		}
	}

	// validate openapi directory

	if err := validateOpenAPIDir(path); err != nil {
		errs = errors.Join(errs, err)
		m.errors.Error(err.Error())
	}

	return errs
}

func parseModuleConfigFile(path string) (*moduleYaml, error) {
	moduleFilename := filepath.Join(path, module.ModuleConfigFilename)
	yamlFile, err := readFile(moduleFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var deckhouseModule moduleYaml
	err = yaml.Unmarshal(yamlFile, &deckhouseModule)
	if err != nil {
		return nil, err
	}

	return &deckhouseModule, nil
}

func parseChartFile(path string) (*chartYaml, error) {
	chartFilename := filepath.Join(path, module.ChartConfigFilename)
	yamlFile, err := readFile(chartFilename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var chart chartYaml
	err = yaml.Unmarshal(yamlFile, &chart)
	if err != nil {
		return nil, err
	}

	return &chart, nil
}

func readFile(filePath string) ([]byte, error) {
	_, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}

func getNamespace(path string) string {
	content, err := os.ReadFile(filepath.Join(path, ".namespace"))
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(content), " \t\n")
}

func validateOpenAPIDir(path string) error {
	openAPIDir := filepath.Join(path, "openapi")
	_, err := os.Stat(openAPIDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("OpenAPI dir does not exist")
		}

		return fmt.Errorf("failed to access OpenAPI dir: %w", err)
	}

	var errs error
	valuesFile := filepath.Join(openAPIDir, "values.yaml")
	_, err = os.Stat(valuesFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, fmt.Errorf("OpenAPI dir does not contain values.yaml"))
		} else {
			errs = errors.Join(errs, fmt.Errorf("failed to access OpenAPI values.yaml: %w", err))
		}
	}

	configValuesFile := filepath.Join(openAPIDir, "config-values.yaml")
	_, err = os.Stat(configValuesFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errs = errors.Join(errs, fmt.Errorf("OpenAPI dir does not contain config-values.yaml"))
		} else {
			errs = errors.Join(errs, fmt.Errorf("failed to access OpenAPI config-values.yaml: %w", err))
		}
	}

	return errs
}
