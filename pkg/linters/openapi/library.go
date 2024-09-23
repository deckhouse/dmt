package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/fsutils"
	"github.com/deckhouse/d8-lint/pkg/linters/openapi/validators"
	"github.com/deckhouse/d8-lint/pkg/logger"

	"github.com/hashicorp/go-multierror"

	"gopkg.in/yaml.v3"
)

const (
	// magic number to limit count of concurrent parses. Way to avoid CPU throttling if it would be huge amount of files
	parserConcurrentCount = 50
)

type fileValidation struct {
	moduleName      string
	filePath        string
	rootPath        string
	validationError error
}

// GetOpenAPIYAMLFiles returns all .yaml files which are placed into openapi/ | crds/ directory
func GetOpenAPIYAMLFiles(rootPath string) ([]string, error) {
	var result []string
	files, err := fsutils.GetFiles(rootPath, false)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !strings.HasSuffix(file, ".yaml") {
			continue
		}

		// ignore matrix test specs
		if strings.HasSuffix(file, "-tests.yaml") {
			continue
		}

		if strings.HasPrefix(filepath.Base(file), "doc-ru-") {
			continue
		}

		arr := strings.Split(file, "/")

		parentDir := arr[len(arr)-2]

		// check only openapi and crds specs
		switch parentDir {
		case "openapi", "crds":
		// pass

		default:
			continue
		}
		p, _ := strings.CutPrefix(file, rootPath)
		result = append(result, p)
	}

	return result, err
}

// RunOpenAPIValidator runs validator, get channel with file paths and returns channel with results
func RunOpenAPIValidator(fileC chan fileValidation, cfg *config.OpenAPISettings) chan fileValidation {
	resultC := make(chan fileValidation, 1)

	go func() {
		for vfile := range fileC {
			parseResultC := make(chan error, parserConcurrentCount)
			yamlStruct := getFileYAMLContent(filepath.Join(vfile.rootPath, vfile.filePath))

			if yamlStruct == nil {
				continue
			}
			runFileParser(vfile.moduleName, vfile.filePath, yamlStruct, cfg, parseResultC)

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
				moduleName:      vfile.moduleName,
				filePath:        vfile.filePath,
				rootPath:        vfile.rootPath,
				validationError: resultErr,
			}
		}

		close(resultC)
	}()

	return resultC
}

type fileParser struct {
	moduleName    string
	fileName      string
	keyValidators map[string]validator

	resultC chan error
}

func getFileYAMLContent(path string) map[any]any {
	data, err := os.ReadFile(path)
	logger.CheckErr(err)

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

func (fp fileParser) parseForWrongKeys(m map[any]any, cfg *config.OpenAPISettings) {
	keysValidator := validators.NewKeyNameValidator(cfg)
	err := keysValidator.Run(fp.fileName, "allfile", m)
	if err != nil {
		fp.resultC <- err
	}
}

func runFileParser(moduleName, fileName string, data map[any]any, cfg *config.OpenAPISettings, resultC chan error) {
	// exclude external CRDs
	if isCRD(data) && !isDeckhouseCRD(data) {
		close(resultC)
		return
	}

	parser := fileParser{
		moduleName: moduleName,
		fileName:   fileName,
		keyValidators: map[string]validator{
			"enum":             validators.NewEnumValidator(cfg),
			"highAvailability": validators.NewHAValidator(cfg),
			"https":            validators.NewHAValidator(cfg),
		},
		resultC: resultC,
	}
	if isDeckhouseCRD(data) {
		parser.parseForWrongKeys(data, cfg)
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
				err := val.Run(fp.moduleName, fp.fileName, absKey, v)
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
	Run(moduleName, fileName, absoluteKey string, value any) error
}
