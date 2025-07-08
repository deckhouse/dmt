/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
