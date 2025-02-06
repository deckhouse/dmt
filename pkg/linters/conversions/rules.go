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
func (c *Conversions) checkModuleYaml(moduleName, modulePath string) {
	_, ok := c.cfg.SkipCheckModule[moduleName]
	if ok {
		return
	}

	configFilePath := filepath.Join(modulePath, configValuesFile)
	_, err := os.Stat(configFilePath)
	if err != nil && os.IsNotExist(err) {
		return
	}

	f, err := os.Open(configFilePath)
	if err != nil {
		c.ErrorList.WithFilePath(configFilePath).
			Criticalf("Cannot open config-values.yaml file: %s", err)

		return
	}

	var cv configValues
	err = yaml.NewDecoder(f).Decode(&cv)
	if err != nil {
		c.ErrorList.WithFilePath(configFilePath).
			Criticalf("Cannot decode config-values.yaml file: %s", err)

		return
	}

	if cv.ConfigVersion == 0 {
		return
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		c.ErrorList.WithFilePath(conversionsFolder).
			Criticalf("Cannot stat conversions folder: %s", err)

		return
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		c.ErrorList.WithFilePath(conversionsFolder).
			Criticalf("Conversions folder is not exist: %s", err)

		return
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			c.ErrorList.Criticalf("Walk error with file: %q", path)

			return nil
		}

		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		// TODO: return error that name is matched and is dir

		conv, err := parseConversion(path)
		if err != nil {
			c.ErrorList.WithFilePath(conversionsFolder).
				Critical(strings.ToTitle(err.Error()))

			return nil
		}

		c.conversionCheck(conv, moduleName, path)

		if conv.Version == nil {
			return nil
		}

		versions = append(versions, *conv.Version)

		c.compareWithFileName(conv, moduleName, path)

		return nil
	})

	if len(versions) == 0 {
		c.ErrorList.Criticalf("No versions in folder: %q", folder)

		return
	}

	slices.Sort(versions)

	if c.cfg.FirstVersion != 0 && versions[0] != c.cfg.FirstVersion {
		c.ErrorList.Criticalf("You need to start with version number: %d", c.cfg.FirstVersion)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			c.ErrorList.Criticalf("No sequential versions between %d and %d", versions[i], versions[i-1])
		}
	}
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

func (c *Conversions) conversionCheck(conv *conversion, moduleName, path string) {
	c.descriptionCheck(conv, moduleName, path)

	if conv.Version == nil {
		c.ErrorList.WithFilePath(path).
			Criticalf("Version is empty, filename: %q", filepath.Base(path))
	}
}

func (c *Conversions) descriptionCheck(conv *conversion, moduleName, path string) {
	if conv.Description == nil {
		c.ErrorList.WithFilePath(path).
			Criticalf("Description is empty, filename: %q", filepath.Base(path))

		return
	}

	if conv.Description.Russian == "" {
		c.ErrorList.WithFilePath(path).
			Criticalf("No description for conversion: russian, filename: %q", filepath.Base(path))
	}

	if conv.Description.English == "" {
		c.ErrorList.WithFilePath(path).
			Criticalf("No description for conversion: english, filename: %q", filepath.Base(path))
	}
}

func (c *Conversions) compareWithFileName(conv *conversion, moduleName, path string) {
	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		c.ErrorList.WithFilePath(path).
			Criticalf("Bad filename %q", filepath.Base(path))

		return
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		c.ErrorList.WithFilePath(path).
			Criticalf("Cannot convert version from file name %q: %s", filepath.Base(path))

		return
	}

	if *conv.Version != fileVersion {
		c.ErrorList.WithFilePath(path).
			Criticalf("File name %q doesn't correspond with contained version %d", filepath.Base(path), *conv.Version)
	}
}
