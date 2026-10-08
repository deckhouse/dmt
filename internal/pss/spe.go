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
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"
)

const speCRDFile = "policies/crds/security-policy-exception.yaml"

// ArrayItem marks "each element of the array" in an allowance path.
const ArrayItem = "[]"

var speAllowancePaths = sync.OnceValues(func() ([][]string, error) {
	data, err := policies.ReadFile(speCRDFile)
	if err != nil {
		return nil, fmt.Errorf("pss: read SecurityPolicyException CRD: %w", err)
	}

	return allowancePaths(data)
})

// SPEAllowancePaths returns the allowances of SecurityPolicyException (SPEAPIVersion)
// as paths relative to spec, taken from the CRD schema: an allowance is every schema
// object with a `metadata` object property, wherever it is. ArrayItem segments stand
// for array elements. Paths are sorted; the result is shared and must not be modified.
func SPEAllowancePaths() ([][]string, error) {
	return speAllowancePaths()
}

type crd struct {
	Spec struct {
		Versions []crdVersion `json:"versions"`
	} `json:"spec"`
}

type crdVersion struct {
	Name   string `json:"name"`
	Served bool   `json:"served"`
	Schema struct {
		OpenAPIV3Schema *schema `json:"openAPIV3Schema"`
	} `json:"schema"`
}

type schema struct {
	Type       string             `json:"type"`
	Properties map[string]*schema `json:"properties"`
	Items      *schema            `json:"items"`
	// Only to refuse a map of objects: the walker cannot name its keys.
	AdditionalProperties any `json:"additionalProperties"`
}

func allowancePaths(data []byte) ([][]string, error) {
	var c crd
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("pss: parse SecurityPolicyException CRD: %w", err)
	}

	version := path.Base(SPEAPIVersion)

	i := slices.IndexFunc(c.Spec.Versions, func(v crdVersion) bool { return v.Name == version })
	if i < 0 || !c.Spec.Versions[i].Served {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD has no served version %s", version)
	}

	root := c.Spec.Versions[i].Schema.OpenAPIV3Schema
	if root == nil || root.Properties["spec"] == nil {
		return nil, errors.New("pss: SecurityPolicyException CRD: no openAPIV3Schema.properties.spec")
	}

	var (
		res [][]string
		bad []string
	)

	var walk func(p []string, s *schema)

	walk = func(p []string, s *schema) {
		if s == nil {
			return
		}

		if _, isMap := s.AdditionalProperties.(map[string]any); isMap {
			bad = append(bad, strings.Join(p, "."))
		}

		if m := s.Properties["metadata"]; m != nil && m.Type == "object" {
			res = append(res, slices.Clone(p))
		}

		for k, child := range s.Properties {
			if k != "metadata" {
				walk(append(p, k), child)
			}
		}

		walk(append(p, ArrayItem), s.Items)
	}

	walk(nil, root.Properties["spec"])

	if len(bad) > 0 {
		return nil, fmt.Errorf("pss: SecurityPolicyException CRD: unsupported additionalProperties schema at spec.%s", strings.Join(bad, ", spec."))
	}

	if len(res) == 0 {
		return nil, errors.New("pss: SecurityPolicyException CRD: no allowances (objects with metadata) found in spec")
	}

	slices.SortFunc(res, func(a, b []string) int { return strings.Compare(strings.Join(a, "."), strings.Join(b, ".")) })

	return res, nil
}
