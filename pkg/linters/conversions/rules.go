package conversions

import (
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
	ConversionsFolder = "openapi/conversions"
)

var regexVersionFile = regexp.MustCompile("v([1-9]|[1-9][0-9]|[1-9][0-9][0-9]).yaml|v([1-9]|[1-9][0-9]|[1-9][0-9][0-9]).yml")

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

	folder := filepath.Join(filepath.Join(modulePath, ConversionsFolder))

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		result.Add(errors.NewLintRuleError(
			ID,
			moduleName,
			moduleName,
			nil,
			"Cannot stat conversions folder %q: %s",
			ConversionsFolder, err.Error(),
		))

		return result
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, info fs.FileInfo, _ error) error {
		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Cannot open file to read conversion %q: %s",
				ConversionsFolder, err.Error(),
			))

			return nil
		}

		c := new(conversion)
		err = yaml.NewDecoder(file).Decode(c)
		if err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Cannot decode yaml %q: %s",
				ConversionsFolder, err.Error(),
			))

			return nil
		}

		if c.Description != nil {
			if c.Description.Russian == "" {
				result.Add(errors.NewLintRuleError(
					ID,
					moduleName,
					moduleName,
					nil,
					"No description for conversion: russian",
				))
			}

			if c.Description.English == "" {
				result.Add(errors.NewLintRuleError(
					ID,
					moduleName,
					moduleName,
					nil,
					"No description for conversion: russian",
				))
			}
		}

		if c.Version == nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Version is empty, filename: %q",
				filepath.Base(path),
			))

			return nil
		}

		versions = append(versions, *c.Version)

		separated := strings.SplitN(filepath.Base(path), ".", 2)
		if len(separated) <= 1 {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Bad filename %q",
				filepath.Base(path),
			))

			return nil
		}

		rawVersion := strings.TrimPrefix(separated[0], "v")

		fileVersion, err := strconv.Atoi(rawVersion)
		if err != nil {
			result.Add(errors.NewLintRuleError(
				ID,
				moduleName,
				moduleName,
				nil,
				"Cannot convert version from file name %q: %s",
				filepath.Base(path), err.Error(),
			))

			return nil
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

			return nil
		}

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

	if versions[0] != Cfg.FirstVersion {
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
