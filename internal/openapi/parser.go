package openapi

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func IsCRD(data map[any]any) bool {
	kind, ok := data["kind"].(string)
	if !ok {
		return false
	}

	if kind != "CustomResourceDefinition" {
		return false
	}

	return true
}

func IsDeckhouseCRD(data map[any]any) bool {
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

type parser func(string, any) error

func Parse(parser parser, path string) error {
	data, err := getFileYAMLContent(path)
	if err != nil {
		return fmt.Errorf("failed to get content of `%s`: %w", path, err)
	}

	// exclude external CRDs
	if IsCRD(data) && !IsDeckhouseCRD(data) {
		return nil
	}

	fp := fileParser{
		parser: parser,
	}

	for k, v := range data {
		err = errors.Join(err, fp.parseValue(k.(string), v))
	}

	return err
}

func getFileYAMLContent(path string) (map[any]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	m := make(map[any]any)

	err = yaml.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

type fileParser struct {
	parser parser
}

func (fp *fileParser) parseMap(upperKey string, m map[any]any) error {
	var err error
	for k, v := range m {
		absKey := fmt.Sprintf("%s.%s", upperKey, k)
		if _, ok := k.(string); ok {
			err = errors.Join(err, fp.parser(absKey, v))
		}
		err = errors.Join(err, fp.parseValue(absKey, v))
	}

	return err
}

func (fp *fileParser) parseSlice(upperKey string, slc []any) error {
	var err error
	for k, v := range slc {
		err = errors.Join(err, fp.parseValue(fmt.Sprintf("%s[%d]", upperKey, k), v))
	}

	return err
}

func (fp *fileParser) parseValue(upperKey string, v any) error {
	if v == nil {
		return nil
	}
	typ := reflect.TypeOf(v).Kind()

	var err error
	switch typ {
	case reflect.Map:
		if m, ok := v.(map[any]any); ok {
			err = errors.Join(err, fp.parseMap(upperKey, m))
		}
		if m, ok := v.(map[string]any); ok {
			nm := make(map[any]any)
			for k, v := range m {
				nm[k] = v
			}
			err = errors.Join(err, fp.parseMap(upperKey, nm))
		}
	case reflect.Slice:
		err = errors.Join(err, fp.parseSlice(upperKey, v.([]any)))
	default:
	}

	return err
}
