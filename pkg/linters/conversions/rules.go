package conversions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	conversionsFolder = "openapi/conversions"
	configValuesFile  = "openapi/config-values.yaml"
)

var regexVersionFile = regexp.MustCompile(`^v([1-9]\d{0,2})\.ya?ml$`)

type conversion struct {
	Version     *int         `yaml:"version,omitempty"`
	Description *description `yaml:"description,omitempty"`
}

type description struct {
	English string `yaml:"en,omitempty"`
	Russian string `yaml:"ru,omitempty"`
}

type configValues struct {
	ConfigVersion int `yaml:"x-config-version"`
}

//nolint:gocyclo // hate this linter
func checkModuleYaml(moduleName, modulePath string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName)

	_, ok := cfg.SkipCheckModule[moduleName]
	if ok {
		return result
	}

	configFilePath := filepath.Join(modulePath, configValuesFile)
	_, err := os.Stat(configFilePath)
	if err != nil && os.IsNotExist(err) {
		return result
	}

	f, err := os.Open(configFilePath)
	if err != nil {
		return result.WithModule(moduleName).
			WithFilePath(configFilePath).
			Add(
				"Cannot open config-values.yaml file: %s",
				err.Error(),
			)
	}

	var cv configValues
	err = yaml.NewDecoder(f).Decode(&cv)
	if err != nil {
		return result.WithModule(moduleName).
			WithFilePath(configFilePath).
			Add(
				"Cannot decode config-values.yaml file: %s",
				err.Error(),
			)
	}

	if cv.ConfigVersion == 0 {
		return result
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		return result.
			WithFilePath(conversionsFolder).
			Add(
				"Cannot stat conversions folder: %s",
				err.Error(),
			)
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		return result.
			WithFilePath(conversionsFolder).
			Add(
				"Conversions folder is not exist: %s",
				err.Error(),
			)
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			result.Add(
				"Walk error with file: %q",
				path,
			)

			return nil
		}

		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		// TODO: return error that name is matched and is dir

		c, err := parseConversion(path)
		if err != nil {
			result.
				WithFilePath(path).
				Add(
					"%s",
					strings.ToTitle(err.Error()),
				)

			return nil
		}

		result.Merge(conversionCheck(c, moduleName, path))

		if c.Version == nil {
			return nil
		}

		versions = append(versions, *c.Version)

		result.Merge(compareWithFileName(c, moduleName, path))

		return nil
	})

	if len(versions) == 0 {
		return result.Add(
			"No versions in folder: %q",
			folder,
		)
	}

	slices.Sort(versions)

	if cfg.FirstVersion != 0 && versions[0] != cfg.FirstVersion {
		result.Add(
			"You need to start with version number: %d",
			cfg.FirstVersion,
		)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			result.Add(
				"No sequential versions between %d and %d",
				versions[i], versions[i-1],
			)
		}
	}

	return result
}

func parseConversion(path string) (*conversion, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file to read conversion %q: %w", conversionsFolder, err)
	}

	c := new(conversion)
	err = yaml.NewDecoder(file).Decode(c)
	if err != nil {
		return nil, fmt.Errorf("cannot decode yaml %q: %w", conversionsFolder, err)
	}

	return c, nil
}

func conversionCheck(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName)

	result.Merge(descriptionCheck(c, moduleName, path))

	if c.Version == nil {
		return result.
			WithFilePath(path).
			Add(
				"Version is empty, filename: %q",
				filepath.Base(path),
			)
	}

	return result
}

func descriptionCheck(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName)

	if c.Description == nil {
		return result.
			WithFilePath(path).
			Add(
				"Description is empty, filename: %q",
				filepath.Base(path),
			)
	}

	if c.Description.Russian == "" {
		result.
			WithFilePath(path).
			Add(
				"No description for conversion: russian, filename: %q",
				filepath.Base(path),
			)
	}

	if c.Description.English == "" {
		result.
			WithFilePath(path).
			Add(
				"No description for conversion: english, filename: %q",
				filepath.Base(path),
			)
	}

	return result
}

func compareWithFileName(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, moduleName)

	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		return result.
			WithFilePath(path).
			Add(
				"Bad filename %q",
				filepath.Base(path),
			)
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		return result.
			WithFilePath(path).
			Add(
				"Cannot convert version from file name %q: %s",
				filepath.Base(path), err.Error(),
			)
	}

	if *c.Version != fileVersion {
		return result.
			WithFilePath(path).
			Add(
				"File name %q doesn't correspond with contained version %d",
				filepath.Base(path), *c.Version,
			)
	}

	return result
}
