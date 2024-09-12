package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/linters/openapi/validators"

	"github.com/hashicorp/go-multierror"

	"gopkg.in/yaml.v3"
)

const (
	// magic number to limit count of concurrent parses. Way to avoid CPU throttling if it would be huge amount of files
	parserConcurrentCount = 50
)

type fileValidation struct {
	filePath string

	rootPath string

	validationError error
}

// GetOpenAPIYAMLFiles returns all .yaml files which are placed into openapi/ | crds/ directory
func GetOpenAPIYAMLFiles(rootPath string) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// ignore matrix test specs
		if strings.HasSuffix(path, "-tests.yaml") {
			return nil
		}

		if strings.HasPrefix(info.Name(), "doc-ru-") {
			return nil
		}

		arr := strings.Split(path, "/")

		parentDir := arr[len(arr)-2]

		// check only openapi and crds specs
		switch parentDir {
		case "openapi", "crds":
		// pass

		default:
			return nil
		}

		result = append(result, path)

		return nil
	})

	return result, err
}

// RunOpenAPIValidator runs validator, get channel with file paths and returns channel with results
func RunOpenAPIValidator(fileC chan fileValidation) chan fileValidation {
	resultC := make(chan fileValidation, 1)

	go func() {
		for vfile := range fileC {
			parseResultC := make(chan error, parserConcurrentCount)

			yamlStruct := getFileYAMLContent(vfile.filePath)

			if yamlStruct == nil {
				continue
			}

			runFileParser(vfile.filePath, yamlStruct, parseResultC)

			var result *multierror.Error

			for err := range parseResultC {
				if err != nil {
					result = multierror.Append(result, err)
				}
			}

			resultErr := result.ErrorOrNil()
			if resultErr == nil {
				continue
			}
			resultC <- fileValidation{
				filePath:        vfile.filePath,
				validationError: resultErr,
			}
		}

		close(resultC)
	}()

	return resultC
}

type fileParser struct {
	fileName      string
	keyValidators map[string]validator

	resultC chan error
}

func getFileYAMLContent(path string) map[any]any {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	m := make(map[any]any)

	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return nil
	}

	return m
}

func isCRD(data map[any]any) bool {
	kind, ok := data["kind"].(string)
	if !ok {
		return false
	}

	if kind != "CustomResourceDefinition" {
		return false
	}

	return true
}

func isDeckhouseCRD(data map[any]any) bool {
	kind, ok := data["kind"].(string)
	if !ok {
		return false
	}

	if kind != "CustomResourceDefinition" {
		return false
	}

	metadata, ok := data["metadata"].(map[any]any)
	if !ok {
		return false
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return false
	}

	if strings.HasSuffix(name, "deckhouse.io") {
		return true
	}

	return false
}

func (fp fileParser) parseForWrongKeys(m map[any]any) {
	keysValidator := validators.NewKeyNameValidator()
	err := keysValidator.Run(fp.fileName, "allfile", m)
	if err != nil {
		fp.resultC <- err
	}
}

func runFileParser(fileName string, data map[any]any, resultC chan error) {
	// exclude external CRDs
	if isCRD(data) && !isDeckhouseCRD(data) {
		close(resultC)
		return
	}

	parser := fileParser{
		fileName: fileName,
		keyValidators: map[string]validator{
			"enum":             validators.NewEnumValidator(),
			"highAvailability": validators.NewHAValidator(),
			"https":            validators.NewHAValidator(),
		},
		resultC: resultC,
	}
	if isDeckhouseCRD(data) {
		parser.parseForWrongKeys(data)
	}
	go parser.startParsing(data, resultC)
}

func (fp fileParser) startParsing(m map[any]any, resultC chan error) {
	for k, v := range m {
		fp.parseValue(k.(string), v)
	}

	close(resultC)
}

func (fp fileParser) parseMap(upperKey string, m map[any]any) {
	for k, v := range m {
		absKey := fmt.Sprintf("%s.%s", upperKey, k)
		if key, ok := k.(string); ok {
			if val, ok := fp.keyValidators[key]; ok {
				err := val.Run(fp.fileName, absKey, v)
				if err != nil {
					fp.resultC <- err
				}
			}
		}
		fp.parseValue(absKey, v)
	}
}

func (fp fileParser) parseSlice(upperKey string, slc []any) {
	for k, v := range slc {
		fp.parseValue(fmt.Sprintf("%s[%d]", upperKey, k), v)
	}
}

func (fp fileParser) parseValue(upperKey string, v any) {
	if v == nil {
		return
	}
	typ := reflect.TypeOf(v).Kind()

	switch typ {
	case reflect.Map:
		if m, ok := v.(map[any]any); ok {
			fp.parseMap(upperKey, m)
		}
		if m, ok := v.(map[string]any); ok {
			nm := make(map[any]any)
			for k, v := range m {
				nm[k] = v
			}
			fp.parseMap(upperKey, nm)
		}
	case reflect.Slice:
		fp.parseSlice(upperKey, v.([]any))
	default:
	}
}

type validator interface {
	Run(fileName, absoluteKey string, value any) error
}
