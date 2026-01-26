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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	OSSRuleName = "oss"
)

func NewOSSRule(disable bool) *OSSRule {
	return &OSSRule{
		RuleMeta: pkg.RuleMeta{
			Name: OSSRuleName,
		},
		BoolRule: pkg.BoolRule{
			Exclude: disable,
		},
	}
}

type OSSRule struct {
	pkg.RuleMeta
	pkg.BoolRule
}

const ossFilename = "oss.yaml"

func (r *OSSRule) OssModuleRule(moduleRoot string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	verifyOssFile(moduleRoot, errorList)
}

func ossFileErrorMessage(err error) string {
	if os.IsNotExist(err) {
		return "Module should have " + ossFilename
	}

	return fmt.Sprintf("Invalid %s: %s", ossFilename, err.Error())
}

func verifyOssFile(moduleRoot string, errorList *errors.LintRuleErrorsList) {
	projects, err := readOssFile(moduleRoot)
	if err != nil {
		errorList.Error(ossFileErrorMessage(err))

		return
	}

	if len(projects) == 0 {
		errorList.Error("no projects described")

		return
	}

	for i, p := range projects {
		assertOssProject(i+1, &p, errorList)
	}
}

func assertOssProject(i int, p *ossProject, errorList *errors.LintRuleErrorsList) {
	// prefix to make it easier navigate among errors
	prefix := fmt.Sprintf("#%d", i)

	// Id
	if strings.TrimSpace(p.Id) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("id must not be empty")
	} else {
		prefix = fmt.Sprintf("#%d (id=%s)", i, p.Id)
	}

	// Version
	if strings.TrimSpace(p.Version) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("version must not be empty. Please fill in the parameter and configure CI (werf files for module images) to use these setting. See ADR \"platform-security/2026-01-21-oss-yaml-werf.md\"")
	} else {
		_, err := semver.NewVersion(p.Version)
		if err != nil {
			errorList.WithObjectID("index=" + prefix + ";").Warn(fmt.Sprintf("version must be valid semver: %v", err))
		}
	}

	// Name
	if strings.TrimSpace(p.Name) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("name must not be empty")
	}

	// Description
	if strings.TrimSpace(p.Description) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("description must not be empty")
	}

	// Link
	if strings.TrimSpace(p.Link) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("link must not be empty")
	} else if _, err := url.ParseRequestURI(p.Link); err != nil {
		errorList.WithObjectID("index=" + prefix + ";").Error(fmt.Sprintf("link URL is malformed (%q)", p.Link))
	}

	// License
	if strings.TrimSpace(p.License) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("License must not be empty")
	}

	// Logo
	if strings.TrimSpace(p.Logo) != "" {
		if _, err := url.ParseRequestURI(p.Logo); err != nil {
			errorList.WithObjectID("index=" + prefix + ";").Error(fmt.Sprintf("project logo URL is malformed (%q)", p.Logo))
		}
	}
}

func readOssFile(moduleRoot string) ([]ossProject, error) {
	b, err := os.ReadFile(filepath.Join(moduleRoot, ossFilename))
	if err != nil {
		return nil, err
	}

	return parseProjectList(b)
}

func parseProjectList(b []byte) ([]ossProject, error) {
	var projects []ossProject
	err := yaml.UnmarshalStrict(b, &projects)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

type ossProject struct {
	Name        string `json:"name"`           // example: Dex
	Description string `json:"description"`    // example: A Federated OpenID Connect Provider with pluggable connectors
	Link        string `json:"link"`           // example: https://github.com/dexidp/dex
	Logo        string `json:"logo,omitempty"` // example: https://dexidp.io/img/logos/dex-horizontal-color.png
	License     string `json:"license"`        // example: Apache License 2.0
	Id          string `json:"id"`             // example: dexidp/dex
	Version     string `json:"version"`        // example: 2.0.0
}
