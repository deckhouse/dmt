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

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"sigs.k8s.io/yaml"
)

type DeckhouseCRDsRule struct {
	pkg.RuleMeta
	pkg.StringRule
	rootPath string
}

const CrdsDir = "crds"

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func NewDeckhouseCRDsRule(cfg *config.OpenAPISettings, rootPath string) *DeckhouseCRDsRule {
	return &DeckhouseCRDsRule{
		RuleMeta: pkg.RuleMeta{
			Name: "deckhouse-crds",
		},
		StringRule: pkg.StringRule{
			ExcludeRules: cfg.OpenAPIExcludeRules.CRDNamesExcludes.Get(),
		},
		rootPath: rootPath,
	}
}

func (r *DeckhouseCRDsRule) Run(moduleName, path string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	shortPath, _ := filepath.Rel(r.rootPath, path)
	fileContent, err := os.ReadFile(path)
	if err != nil {
		errorList.Errorf("Can't read file %s: %s", shortPath, err)
		return
	}

	docs := splitManifests(string(fileContent))
	for _, d := range docs {
		var crd v1beta1.CustomResourceDefinition

		if err := yaml.Unmarshal([]byte(d), &crd); err != nil {
			errorList.Errorf("Can't parse manifests in %s folder: %s", CrdsDir, err)

			continue
		}

		if !strings.Contains(crd.Name, "deckhouse.io") {
			continue
		}

		if crd.APIVersion != "apiextensions.k8s.io/v1" {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
				WithFilePath(shortPath).
				WithValue(crd.APIVersion).
				Errorf(`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`)
		}

		if !r.Enabled(crd.Name) {
			continue
		}

		if heritage, ok := crd.Labels["heritage"]; ok {
			if heritage != "deckhouse" {
				errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
					WithFilePath(shortPath).
					WithValue("heritage = "+heritage).
					Errorf(`CRD should contain "heritage = deckhouse" label, but got "heritage = %s"`, heritage)
			}
		} else {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
				WithFilePath(shortPath).
				WithValue("heritage = deckhouse").
				Errorf(`CRD should contain "heritage = deckhouse" label`)
		}

		if name, ok := crd.Labels["module"]; ok {
			if name != moduleName {
				errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
					WithFilePath(shortPath).
					WithValue("module = "+name).
					Errorf(`CRD should contain "module = %s" label, but got "module = %s"`, moduleName, name)
			}
		} else {
			errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s", crd.Kind, crd.Name)).
				WithFilePath(shortPath).
				WithValue("module = "+moduleName).
				Errorf(`CRD should contain "module = %s" label`, moduleName)
		}
	}
}

func splitManifests(bigFile string) []string {
	bigFileTmp := strings.TrimSpace(bigFile)
	return sep.Split(bigFileTmp, -1)
}
