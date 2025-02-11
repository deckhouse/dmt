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
func (o *Conversions) checkModuleYaml(modulePath string, lintError *errors.Error) {
	configFilePath := filepath.Join(modulePath, configValuesFile)
	_, err := os.Stat(configFilePath)
	if err != nil && os.IsNotExist(err) {
		return
	}

	f, err := os.Open(configFilePath)
	if err != nil {
		lintError.Add(
			"Cannot open config-values.yaml file at path %q: %s",
			configFilePath, err.Error(),
		)
		return
	}

	var cv configValues
	err = yaml.NewDecoder(f).Decode(&cv)
	if err != nil {
		lintError.Add(
			"Cannot decode config-values.yaml file: %s",
			err.Error(),
		)
		return
	}

	if cv.ConfigVersion == 0 {
		return
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		lintError.Add(
			"Cannot stat conversions folder %q: %s",
			conversionsFolder, err.Error(),
		)
		return
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		lintError.Add(
			"Conversions folder is not exist, at path %q: %s",
			conversionsFolder, err.Error(),
		)
		return
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			lintError.Add(
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
			lintError.Add(
				"%s",
				strings.ToTitle(err.Error()),
			)

			return nil
		}

		conversionCheck(c, path, lintError)

		if c.Version == nil {
			return nil
		}

		versions = append(versions, *c.Version)

		compareWithFileName(c, path, lintError)

		return nil
	})

	if len(versions) == 0 {
		lintError.Add(
			"No versions in folder: %q",
			folder,
		)
		return
	}

	slices.Sort(versions)

	if o.cfg.FirstVersion != 0 && versions[0] != o.cfg.FirstVersion {
		lintError.Add(
			"You need to start with version number: %d",
			o.cfg.FirstVersion,
		)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			lintError.Add(
				"No sequential versions between %d and %d",
				versions[i], versions[i-1],
			)
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

func conversionCheck(c *conversion, path string, lintError *errors.Error) {
	descriptionCheck(c, path, lintError)

	if c.Version == nil {
		lintError.Add(
			"Version is empty, filename: %q",
			filepath.Base(path),
		)
	}
}

func descriptionCheck(c *conversion, path string, lintError *errors.Error) {
	if c.Description == nil {
		lintError.Add(
			"Description is empty, filename: %q",
			filepath.Base(path),
		)
		return
	}

	if c.Description.Russian == "" {
		lintError.Add(
			"No description for conversion: russian, filename: %q",
			filepath.Base(path),
		)
	}

	if c.Description.English == "" {
		lintError.Add(
			"No description for conversion: english, filename: %q",
			filepath.Base(path),
		)
	}
}

func compareWithFileName(c *conversion, path string, lintError *errors.Error) {
	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		lintError.Add(
			"Bad filename %q",
			filepath.Base(path),
		)
		return
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		lintError.Add(
			"Cannot convert version from file name %q: %s",
			filepath.Base(path), err.Error(),
		)
		return
	}

	if *c.Version != fileVersion {
		lintError.Add(
			"File name %q doesn't correspond with contained version %d",
			filepath.Base(path), *c.Version,
		)
		return
	}
}
