package schema

import (
	"github.com/go-openapi/spec"
)

type RequiredForHelmTransformer struct{}

const XRequiredForHelm = "x-required-for-helm"

func (*RequiredForHelmTransformer) Transform(s *spec.Schema) *spec.Schema {
	if s == nil {
		return nil
	}

	s.Required = MergeRequiredFields(s.Extensions, s.Required)

	// Deep transform.
	transformRequired(s.Properties)
	return s
}

func transformRequired(props map[string]spec.Schema) {
	for k := range props {
		prop := props[k]
		prop.Required = MergeRequiredFields(prop.Extensions, prop.Required)
		transformRequired(prop.Properties)
	}
}

func MergeArrays(ar1, ar2 []string) []string {
	res := make([]string, 0)
	m := make(map[string]struct{})
	for _, item := range ar1 {
		res = append(res, item)
		m[item] = struct{}{}
	}
	for _, item := range ar2 {
		if _, ok := m[item]; !ok {
			res = append(res, item)
		}
	}
	return res
}

func MergeRequiredFields(ext spec.Extensions, required []string) []string {
	var xReqFields []string
	_, hasField := ext[XRequiredForHelm]
	if !hasField {
		return required
	}
	field, ok := ext.GetString(XRequiredForHelm)
	if ok {
		xReqFields = []string{field}
	} else {
		xReqFields, _ = ext.GetStringSlice(XRequiredForHelm)
	}
	// Merge x-required with required
	return MergeArrays(required, xReqFields)
}
