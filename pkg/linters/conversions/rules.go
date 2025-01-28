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

func checkModuleYaml(moduleName, modulePath string) errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	if slices.Contains(Cfg.SkipCheckModule, moduleName) {
		return result
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot stat conversions folder %q: %s",
			conversionsFolder, err.Error(),
		))

		return result
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Conversions folder is not exist, at path %q: %s",
			conversionsFolder, err.Error(),
		))

		return result
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Walk error with file: %q",
				path,
			))

			return nil
		}

		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		c, err := parseConversion(path)
		if err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"%s",
				strings.ToTitle(err.Error()),
			))

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
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"No versions in folder: %q",
			folder,
		))

		return result
	}

	slices.Sort(versions)

	if Cfg.FirstVersion != 0 && versions[0] != Cfg.FirstVersion {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"You need to start with version number: %d",
			Cfg.FirstVersion,
		))
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"No sequential versions between %d and %d",
				versions[i], versions[i-1],
			))
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

func conversionCheck(c *conversion, moduleName, path string) errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	result.Merge(descriptionCheck(c, moduleName, path))

	if c.Version == nil {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Version is empty, filename: %q",
			filepath.Base(path),
		))

		return result
	}

	return result
}

func descriptionCheck(c *conversion, moduleName, path string) errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	if c.Description == nil {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Description is empty, filename: %q",
			filepath.Base(path),
		))

		return result
	}

	if c.Description.Russian == "" {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"No description for conversion: russian, filename: %q",
			filepath.Base(path),
		))
	}

	if c.Description.English == "" {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"No description for conversion: russian, filename: %q",
			filepath.Base(path),
		))
	}

	return result
}

func compareWithFileName(c *conversion, moduleName, path string) errors.LintRuleErrorsList {
	result := errors.LintRuleErrorsList{}

	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Bad filename %q",
			filepath.Base(path),
		))

		return result
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot convert version from file name %q: %s",
			filepath.Base(path), err.Error(),
		))

		return result
	}

	if *c.Version != fileVersion {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"File name %q doesn't correspond with contained version %d",
			filepath.Base(path), *c.Version,
		))

		return result
	}

	return result
}
