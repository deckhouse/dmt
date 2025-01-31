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

func (o *Conversions) checkModuleYaml(moduleName, modulePath string) *errors.LintRuleErrorsList {
	_, ok := o.cfg.SkipCheckModule[moduleName]
	if ok {
		o.result.WithWarning(true)
	}

	folder := filepath.Join(modulePath, conversionsFolder)

	stat, err := os.Stat(folder)
	if err != nil && !os.IsNotExist(err) {
		return o.result.WithObjectID(moduleName).Add(
			"Cannot stat conversions folder %q: %s",
			conversionsFolder, err.Error(),
		)
	}

	if os.IsNotExist(err) || !stat.IsDir() {
		return o.result.WithObjectID(moduleName).Add(
			"Conversions folder is not exist, at path %q: %s",
			conversionsFolder, err.Error(),
		)
	}

	versions := make([]int, 0)

	_ = filepath.Walk(folder, func(path string, _ fs.FileInfo, err error) error {
		if err != nil {
			o.result.WithObjectID(moduleName).Add(
				"Walk error with file: %q",
				path,
			)

			return nil
		}

		if !regexVersionFile.MatchString(filepath.Base(path)) {
			return nil
		}

		// TODO: return error that name is matched and is dir

		c, err := o.parseConversion(path)
		if err != nil {
			o.result.WithObjectID(moduleName).Add(
				"%s",
				strings.ToTitle(err.Error()),
			)

			return nil
		}

		o.result.Merge(o.conversionCheck(c, moduleName, path))

		if c.Version == nil {
			return nil
		}

		versions = append(versions, *c.Version)

		o.result.Merge(o.compareWithFileName(c, moduleName, path))

		return nil
	})

	if len(versions) == 0 {
		return o.result.WithObjectID(moduleName).Add(
			"No versions in folder: %q",
			folder,
		)
	}

	slices.Sort(versions)

	if o.cfg.FirstVersion != 0 && versions[0] != o.cfg.FirstVersion {
		o.result.WithObjectID(moduleName).Add(
			"You need to start with version number: %d",
			o.cfg.FirstVersion,
		)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i]-versions[i-1] > 1 {
			o.result.WithObjectID(moduleName).Add(
				"No sequential versions between %d and %d",
				versions[i], versions[i-1],
			)
		}
	}

	return o.result
}

func (o *Conversions) parseConversion(path string) (*conversion, error) {
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

func (o *Conversions) conversionCheck(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	o.result.Merge(o.descriptionCheck(c, moduleName, path))

	if c.Version == nil {
		return o.result.WithObjectID(moduleName).Add(
			"Version is empty, filename: %q",
			filepath.Base(path),
		)
	}

	return o.result
}

func (o *Conversions) descriptionCheck(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	if c.Description == nil {
		return o.result.WithObjectID(moduleName).Add(
			"Description is empty, filename: %q",
			filepath.Base(path),
		)
	}

	if c.Description.Russian == "" {
		o.result.WithObjectID(moduleName).Add(
			"No description for conversion: russian, filename: %q",
			filepath.Base(path),
		)
	}

	if c.Description.English == "" {
		o.result.WithObjectID(moduleName).Add(
			"No description for conversion: english, filename: %q",
			filepath.Base(path),
		)
	}

	return o.result
}

func (o *Conversions) compareWithFileName(c *conversion, moduleName, path string) *errors.LintRuleErrorsList {
	versions := regexVersionFile.FindStringSubmatch(filepath.Base(path))
	if len(versions) <= 1 {
		return o.result.WithObjectID(moduleName).Add(
			"Bad filename %q",
			filepath.Base(path),
		)
	}

	fileVersion, err := strconv.Atoi(versions[1])
	if err != nil {
		return o.result.WithObjectID(moduleName).Add(
			"Cannot convert version from file name %q: %s",
			filepath.Base(path), err.Error(),
		)
	}

	if *c.Version != fileVersion {
		return o.result.WithObjectID(moduleName).Add(
			"File name %q doesn't correspond with contained version %d",
			filepath.Base(path), *c.Version,
		)
	}

	return o.result
}
