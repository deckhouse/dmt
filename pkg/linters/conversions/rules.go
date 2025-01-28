package conversions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"gopkg.in/yaml.v2"

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

func (c *Conversions) checkModuleYaml(moduleName, modulePath string) *errors.LintRuleErrorsList {
	errList := c.lintErrors.WithLinterID(ID).WithModuleID(moduleName).WithObjectID(moduleName)

	_, ok := c.cfg.SkipCheckModule[moduleName]
	if ok {
		return errList
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		return errList.AddF("Cannot stat conversions folder %q: %s", conversionsFolder, err)
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		return errList.AddF("Conversions folder is not exist, at path %q: %s", conversionsFolder, err)
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			errList.AddF("Walk error with file: %q", path)

			return nil
		}

		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		// TODO: return error that name is matched and is dir

		c, err := parseConversion(path)
		if err != nil {
			errList.AddErr(err)

			return nil
		}

		conversionCheck(c, path, errList)

		if c.Version == nil {
			return nil
		}

		versions = append(versions, *c.Version)

		compareWithFileName(c, path, errList)

		return nil
	})

	if len(versions) == 0 {
		return errList.AddF("No versions in folder: %q", folder)
	}

	slices.Sort(versions)

	if c.cfg.FirstVersion != 0 && versions[0] != c.cfg.FirstVersion {
		errList.AddF("You need to start with version number: %d", c.cfg.FirstVersion)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			errList.AddF("No sequential versions between %d and %d", versions[i], versions[i-1])
		}
	}

	return errList
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

func conversionCheck(c *conversion, path string, errList *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	descriptionCheck(c, path, errList)

	if c.Version == nil {
		return errList.AddF("Version is empty, filename: %q", filepath.Base(path))
	}

	return errList
}

func descriptionCheck(c *conversion, path string, errList *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	if c.Description == nil {
		return errList.AddF("Description is empty, filename: %q", filepath.Base(path))
	}

	if c.Description.Russian == "" {
		errList.AddF("No description for conversion: russian, filename: %q", filepath.Base(path))
	}

	if c.Description.English == "" {
		errList.AddF("No description for conversion: english, filename: %q", filepath.Base(path))
	}

	return errList
}

func compareWithFileName(c *conversion, path string, errList *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		return errList.AddF("Bad filename %q", filepath.Base(path))
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		return errList.AddF("Cannot convert version from file name %q: %s", filepath.Base(path), err)
	}

	if *c.Version != fileVersion {
		return errList.AddF("File name %q doesn't correspond with contained version %d", filepath.Base(path), *c.Version)
	}

	return errList
}
