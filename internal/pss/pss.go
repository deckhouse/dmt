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

// Package pss renders Pod Security Standards policies of admission-policy-engine
// (synced into policies/) into a list of checks executable with rego.
package pss

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"sigs.k8s.io/yaml"
)

const (
	Baseline   = "baseline"
	Restricted = "restricted"
)

// policies/ is owned by the sync script. "all:" is required: without it embed skips _rego-libs.tpl.
//
//go:embed all:policies
var policies embed.FS

//go:embed stub.tpl
var stub []byte

const (
	ctChartDir     = "policies/charts/constraint-templates"
	paramsDir      = "policies/templates/policies/pod-security-standards"
	paramsChart    = "pss"
	constraintFile = "constraint.yaml"
)

// Check is one PSS constraint: kind with its parameters and the rego of its ConstraintTemplate.
// One kind may appear in both standards with different parameters, each is a separate Check.
type Check struct {
	Standard   string // Baseline or Restricted
	Kind       string
	Parameters map[string]any
	Libs       []string
	Rego       string
}

var checks = sync.OnceValues(render)

// Checks returns PSS checks of both standards: baseline first, then restricted.
// Policies are rendered once; the returned slice is shared and must not be modified.
func Checks() ([]Check, error) {
	return checks()
}

type regoSource struct {
	Libs []string `json:"libs"`
	Rego string   `json:"rego"`
}

func render() ([]Check, error) {
	templates, err := renderConstraintTemplates()
	if err != nil {
		return nil, err
	}

	params, err := renderParams()
	if err != nil {
		return nil, err
	}

	var res []Check

	for _, std := range []string{Baseline, Restricted} {
		for _, p := range params[std] {
			src, ok := templates[p.Kind]
			if !ok {
				return nil, fmt.Errorf("pss %s: no ConstraintTemplate for kind %q", std, p.Kind)
			}

			res = append(res, Check{
				Standard:   std,
				Kind:       p.Kind,
				Parameters: p.Parameters,
				Libs:       src.Libs,
				Rego:       src.Rego,
			})
		}
	}

	return res, nil
}

type constraintTemplate struct {
	Spec struct {
		CRD struct {
			Spec struct {
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
			} `json:"spec"`
		} `json:"crd"`
		Targets []struct {
			Code []struct {
				Engine string     `json:"engine"`
				Source regoSource `json:"source"`
			} `json:"code"`
		} `json:"targets"`
	} `json:"spec"`
}

// renderConstraintTemplates returns rego sources by spec.crd.spec.names.kind.
func renderConstraintTemplates() (map[string]regoSource, error) {
	c := newChart("constraint-templates")

	err := fs.WalkDir(policies, ctChartDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		data, err := policies.ReadFile(p)
		if err != nil {
			return err
		}

		f := &chart.File{Name: strings.TrimPrefix(p, ctChartDir+"/"), Data: data}
		if strings.HasPrefix(f.Name, "templates/") {
			c.Templates = append(c.Templates, f)
		} else {
			c.Files = append(c.Files, f)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pss: read constraint templates: %w", err)
	}

	out, err := renderChart(c, nil)
	if err != nil {
		return nil, fmt.Errorf("pss: render constraint templates: %w", err)
	}

	res := make(map[string]regoSource)

	for name, manifest := range out {
		if strings.HasPrefix(path.Base(name), "_") {
			continue
		}

		var ct constraintTemplate
		if err := yaml.Unmarshal([]byte(manifest), &ct); err != nil {
			return nil, fmt.Errorf("pss: parse %s: %w", name, err)
		}

		kind := ct.Spec.CRD.Spec.Names.Kind
		if kind == "" {
			return nil, fmt.Errorf("pss: %s: empty spec.crd.spec.names.kind", name)
		}

		if _, dup := res[kind]; dup {
			return nil, fmt.Errorf("pss: %s: duplicate ConstraintTemplate for kind %q", name, kind)
		}

		for _, t := range ct.Spec.Targets {
			for _, code := range t.Code {
				if code.Engine == "Rego" {
					res[kind] = code.Source
				}
			}
		}

		if res[kind].Rego == "" {
			return nil, fmt.Errorf("pss: %s: no Rego code for kind %q", name, kind)
		}
	}

	return res, nil
}

type kindParams struct {
	Kind       string         `json:"kind"`
	Parameters map[string]any `json:"parameters"`
}

// renderParams renders both constraint.yaml with the stub instead of the original _helpers.tpl.
func renderParams() (map[string][]kindParams, error) {
	c := newChart(paramsChart)
	c.Templates = append(c.Templates, &chart.File{Name: "templates/_dmt_pss_stub.tpl", Data: stub})

	for _, std := range []string{Baseline, Restricted} {
		data, err := policies.ReadFile(path.Join(paramsDir, std, constraintFile))
		if err != nil {
			return nil, fmt.Errorf("pss: read %s params: %w", std, err)
		}

		c.Templates = append(c.Templates, &chart.File{Name: path.Join("templates", std, constraintFile), Data: data})
	}

	// Empty podSecurityStandards (no knownRanges) is the cluster default and gives D8HostNetwork `ranges: []`.
	values := map[string]any{
		"admissionPolicyEngine": map[string]any{
			"podSecurityStandards": map[string]any{},
			"internal": map[string]any{
				"podSecurityStandards": map[string]any{"enforcementActions": []any{"deny"}},
			},
		},
	}

	out, err := renderChart(c, values)
	if err != nil {
		return nil, fmt.Errorf("pss: render params: %w", err)
	}

	res := make(map[string][]kindParams)

	for _, std := range []string{Baseline, Restricted} {
		var list []kindParams
		if err := yaml.Unmarshal([]byte(out[path.Join(paramsChart, "templates", std, constraintFile)]), &list); err != nil {
			return nil, fmt.Errorf("pss: parse %s params: %w", std, err)
		}

		if len(list) == 0 {
			return nil, fmt.Errorf("pss: no %s checks rendered", std)
		}

		res[std] = list
	}

	return res, nil
}

func newChart(name string) *chart.Chart {
	return &chart.Chart{Metadata: &chart.Metadata{Name: name, Version: "0.0.1", APIVersion: chart.APIVersionV2}}
}

func renderChart(c *chart.Chart, values map[string]any) (map[string]string, error) {
	rv, err := chartutil.ToRenderValues(c, values, chartutil.ReleaseOptions{Name: c.Name()}, chartutil.DefaultCapabilities)
	if err != nil {
		return nil, err
	}

	return engine.Render(c, rv)
}
