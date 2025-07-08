package schema

import (
	"encoding/json"

	"maps"

	"github.com/go-openapi/spec"
)

const XExtendKey = "x-extend"

type ExtendSettings struct {
	Schema *string `json:"schema,omitempty"`
}

type ExtendTransformer struct {
	Parent *spec.Schema
}

func (t *ExtendTransformer) Transform(s *spec.Schema) *spec.Schema {
	if t.Parent == nil {
		return s
	}

	extendSettings := ExtractExtendSettings(s)

	if extendSettings == nil || extendSettings.Schema == nil {
		return s
	}

	// TODO check extendSettings.Schema. No need to do it for now.

	s.Definitions = mergeDefinitions(s, t.Parent)
	s.Extensions = mergeExtensions(s, t.Parent)
	s.Required = mergeRequired(s, t.Parent)
	s.Properties = mergeProperties(s, t.Parent)
	s.PatternProperties = mergePatternProperties(s, t.Parent)
	s.Title = mergeTitle(s, t.Parent)
	s.Description = mergeDescription(s, t.Parent)

	return s
}

func ExtractExtendSettings(s *spec.Schema) *ExtendSettings {
	if s == nil {
		return nil
	}

	extendSettingsObj, ok := s.Extensions[XExtendKey]
	if !ok {
		return nil
	}

	if extendSettingsObj == nil {
		return nil
	}

	tmpBytes, _ := json.Marshal(extendSettingsObj)

	res := new(ExtendSettings)

	_ = json.Unmarshal(tmpBytes, res)
	return res
}

func mergeRequired(s, parent *spec.Schema) []string {
	res := make([]string, 0)
	resIdx := make(map[string]struct{})

	for _, name := range parent.Required {
		res = append(res, name)
		resIdx[name] = struct{}{}
	}

	for _, name := range s.Required {
		if _, ok := resIdx[name]; !ok {
			res = append(res, name)
		}
	}

	return res
}

// mergeSchemaMaps merges two map[string]spec.Schema maps, with child values overriding parent values
func mergeSchemaMaps(parent, child map[string]spec.Schema) map[string]spec.Schema {
	res := make(map[string]spec.Schema)
	maps.Copy(res, parent)
	maps.Copy(res, child)
	return res
}

func mergeProperties(s, parent *spec.Schema) map[string]spec.Schema {
	return mergeSchemaMaps(parent.Properties, s.Properties)
}

func mergePatternProperties(s, parent *spec.Schema) map[string]spec.Schema {
	return mergeSchemaMaps(parent.PatternProperties, s.PatternProperties)
}

func mergeDefinitions(s, parent *spec.Schema) spec.Definitions {
	res := make(spec.Definitions)
	maps.Copy(res, parent.Definitions)
	maps.Copy(res, s.Definitions)
	return res
}

func mergeExtensions(s, parent *spec.Schema) spec.Extensions {
	ext := make(spec.Extensions)

	for k, v := range parent.Extensions {
		ext.Add(k, v)
	}
	for k, v := range s.Extensions {
		ext.Add(k, v)
	}

	return ext
}

func mergeTitle(s, parent *spec.Schema) string {
	if s.Title != "" {
		return s.Title
	}
	return parent.Title
}

func mergeDescription(s, parent *spec.Schema) string {
	if s.Description != "" {
		return s.Description
	}
	return parent.Description
}
