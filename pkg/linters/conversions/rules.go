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

func (o *Conversions) checkModuleYaml(moduleName, modulePath string, result *errors.LintRuleErrorsList) {
	_, ok := o.cfg.SkipCheckModule[moduleName]
	if ok {
		result.WithWarning(true)
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		result.WithObjectID(moduleName).Add(
			"Cannot stat conversions folder %q: %s",
			conversionsFolder, err.Error(),
		)
		return
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		result.WithObjectID(moduleName).Add(
			"Conversions folder is not exist, at path %q: %s",
			conversionsFolder, err.Error(),
		)
		return
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			result.WithObjectID(moduleName).Add(
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
			result.WithObjectID(moduleName).Add(
				"%s",
				strings.ToTitle(err.Error()),
			)

			return nil
		}

		conversionCheck(c, moduleName, path, result)

		if c.Version == nil {
			return nil
		}

		versions = append(versions, *c.Version)

		compareWithFileName(c, moduleName, path, result)

		return nil
	})

	if len(versions) == 0 {
		result.WithObjectID(moduleName).Add(
			"No versions in folder: %q",
			folder,
		)
		return
	}

	slices.Sort(versions)

	if o.cfg.FirstVersion != 0 && versions[0] != o.cfg.FirstVersion {
		result.WithObjectID(moduleName).Add(
			"You need to start with version number: %d",
			o.cfg.FirstVersion,
		)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			result.WithObjectID(moduleName).Add(
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

func conversionCheck(c *conversion, moduleName, path string, result *errors.LintRuleErrorsList) {
	descriptionCheck(c, moduleName, path, result)

	if c.Version == nil {
		result.WithObjectID(moduleName).Add(
			"Version is empty, filename: %q",
			filepath.Base(path),
		)
		return
	}
}

func descriptionCheck(c *conversion, moduleName, path string, result *errors.LintRuleErrorsList) {
	if c.Description == nil {
		result.WithObjectID(moduleName).Add(
			"Description is empty, filename: %q",
			filepath.Base(path),
		)
		return
	}

	if c.Description.Russian == "" {
		result.WithObjectID(moduleName).Add(
			"No description for conversion: russian, filename: %q",
			filepath.Base(path),
		)
	}

	if c.Description.English == "" {
		result.WithObjectID(moduleName).Add(
			"No description for conversion: english, filename: %q",
			filepath.Base(path),
		)
	}
}

func compareWithFileName(c *conversion, moduleName, path string, result *errors.LintRuleErrorsList) {
	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		result.WithObjectID(moduleName).Add(
			"Bad filename %q",
			filepath.Base(path),
		)
		return
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		result.WithObjectID(moduleName).Add(
			"Cannot convert version from file name %q: %s",
			filepath.Base(path), err.Error(),
		)
		return
	}

	if *c.Version != fileVersion {
		result.WithObjectID(moduleName).Add(
			"File name %q doesn't correspond with contained version %d",
			filepath.Base(path), *c.Version,
		)
		return
	}
}
