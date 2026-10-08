/*
Copyright 2026 Flant JSC

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

package pss

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/yaml"
)

const speCRDFile = "policies/crds/security-policy-exception.yaml"

// ArrayItem marks "each element of the array" in an allowance path.
const ArrayItem = "[]"

// speCRD is the SecurityPolicyException CRD schema, parsed once.
var speCRD = sync.OnceValues(func() (*speSchema, error) {
	data, err := policies.ReadFile(speCRDFile)
	if err != nil {
		return nil, fmt.Errorf("pss: read SecurityPolicyException CRD: %w", err)
	}

	root, err := parseCRD(data)
	if err != nil {
		return nil, err
	}

	paths, err := allowancePathsOf(root)
	if err != nil {
		return nil, err
	}

	structural, err := structuralschema.NewStructural(root)
	if err != nil {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD: structural schema: %w", err)
	}

	return &speSchema{paths: paths, structural: structural}, nil
})

type speSchema struct {
	paths      [][]string
	structural *structuralschema.Structural
}

// SPEAllowancePaths returns the allowances of SecurityPolicyException (SPEAPIVersion)
// as paths relative to spec, taken from the CRD schema: an allowance is every schema
// object with a `metadata` object property, wherever it is. ArrayItem segments stand
// for array elements. Paths are sorted; the result is shared and must not be modified.
func SPEAllowancePaths() ([][]string, error) {
	s, err := speCRD()
	if err != nil {
		return nil, err
	}

	return s.paths, nil
}

// DefaultSPE returns a copy of a SecurityPolicyException object with the `default`s of
// the CRD schema applied, as the apiserver stores it.
func DefaultSPE(obj map[string]any) (map[string]any, error) {
	s, err := speCRD()
	if err != nil {
		return nil, err
	}

	res, err := jsonCopy(obj)
	if err != nil {
		return nil, err
	}

	defaulting.Default(res, s.structural)

	return res, nil
}

// jsonCopy deep-copies obj into the types the apiserver decodes JSON into (int64 for
// integers), which the schema code expects.
func jsonCopy(obj map[string]any) (map[string]any, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("pss: copy SecurityPolicyException: %w", err)
	}

	var res map[string]any
	if err := utiljson.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("pss: copy SecurityPolicyException: %w", err)
	}

	return res, nil
}

// parseCRD returns the openAPIV3Schema of the SPEAPIVersion version of the CRD.
func parseCRD(data []byte) (*apiextensions.JSONSchemaProps, error) {
	var c apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("pss: parse SecurityPolicyException CRD: %w", err)
	}

	version := path.Base(SPEAPIVersion)

	i := slices.IndexFunc(c.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool { return v.Name == version })
	if i < 0 || !c.Spec.Versions[i].Served {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD has no served version %s", version)
	}

	v := c.Spec.Versions[i]
	if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
		return nil, errors.New("pss: SecurityPolicyException CRD: no openAPIV3Schema")
	}

	root := &apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v.Schema.OpenAPIV3Schema, root, nil); err != nil {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD: convert schema: %w", err)
	}

	return root, nil
}

func allowancePaths(data []byte) ([][]string, error) {
	root, err := parseCRD(data)
	if err != nil {
		return nil, err
	}

	return allowancePathsOf(root)
}

func allowancePathsOf(root *apiextensions.JSONSchemaProps) ([][]string, error) {
	spec, ok := root.Properties["spec"]
	if !ok {
		return nil, errors.New("pss: SecurityPolicyException CRD: no openAPIV3Schema.properties.spec")
	}

	var (
		res [][]string
		bad []string
	)

	var walk func(p []string, s *apiextensions.JSONSchemaProps)

	walk = func(p []string, s *apiextensions.JSONSchemaProps) {
		if s == nil {
			return
		}

		// A map of objects: the walker cannot name its keys.
		if s.AdditionalProperties != nil && s.AdditionalProperties.Schema != nil {
			bad = append(bad, strings.Join(p, "."))
		}

		if m, ok := s.Properties["metadata"]; ok && m.Type == "object" {
			res = append(res, slices.Clone(p))
		}

		for k, child := range s.Properties {
			if k != "metadata" {
				walk(append(p, k), &child)
			}
		}

		if s.Items != nil {
			walk(append(p, ArrayItem), s.Items.Schema)
		}
	}

	walk(nil, &spec)

	if len(bad) > 0 {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD: unsupported additionalProperties schema at spec.%s", strings.Join(bad, ", spec."))
	}

	if len(res) == 0 {
		return nil, errors.New("pss: SecurityPolicyException CRD: no allowances (objects with metadata) found in spec")
	}

	slices.SortFunc(res, func(a, b []string) int { return strings.Compare(strings.Join(a, "."), strings.Join(b, ".")) })

	return res, nil
}
