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

	"sigs.k8s.io/yaml"

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
	errorList = errorList.WithRule(r.GetName()).WithEnabled(func() bool {
		return r.Enabled()
	})

	if errs := verifyOssFile(moduleRoot); len(errs) > 0 {
		for _, err := range errs {
			errorList.WithFilePath(moduleRoot).Error(ossFileErrorMessage(err))
		}
	}
}

func ossFileErrorMessage(err error) string {
	if os.IsNotExist(err) {
		return "Module should have " + ossFilename
	}

	return fmt.Sprintf("Invalid %s: %s", ossFilename, err.Error())
}

func verifyOssFile(moduleRoot string) []error {
	projects, err := readOssFile(moduleRoot)
	if err != nil {
		return []error{err}
	}

	if len(projects) == 0 {
		return []error{fmt.Errorf("no projects described")}
	}

	var errs []error
	for i, p := range projects {
		err = assertOssProject(i+1, &p)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func assertOssProject(i int, p *ossProject) error {
	var complaints []string

	// prefix to make it easier navigate among errors
	prefix := fmt.Sprintf("#%d", i)

	// Name

	if strings.TrimSpace(p.Name) == "" {
		complaints = append(complaints, "name must not be empty")
	} else {
		prefix = fmt.Sprintf("#%d (name=%s)", i, p.Name)
	}

	// Description

	if strings.TrimSpace(p.Description) == "" {
		complaints = append(complaints, "description must not be empty")
	}

	// Link

	if strings.TrimSpace(p.Link) == "" {
		complaints = append(complaints, "link must not be empty")
	} else if _, err := url.ParseRequestURI(p.Link); err != nil {
		complaints = append(complaints, fmt.Sprintf("link URL is malformed (%q)", p.Link))
	}

	// License

	if strings.TrimSpace(p.License) == "" {
		complaints = append(complaints, "License must not be empty")
	}

	// Logo

	if strings.TrimSpace(p.Logo) != "" {
		if _, err := url.ParseRequestURI(p.Logo); err != nil {
			complaints = append(complaints, fmt.Sprintf("project logo URL is malformed (%q)", p.Logo))
		}
	}

	if len(complaints) > 0 {
		return fmt.Errorf("%s: %s", prefix, strings.Join(complaints, "; "))
	}

	return nil
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
	err := yaml.Unmarshal(b, &projects)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

type ossProject struct {
	Name        string `yaml:"name"`           // example: Dex
	Description string `yaml:"description"`    // example: A Federated OpenID Connect Provider with pluggable connectors
	Link        string `yaml:"link"`           // example: https://github.com/dexidp/dex
	Logo        string `yaml:"logo,omitempty"` // example: https://dexidp.io/img/logos/dex-horizontal-color.png
	License     string `yaml:"license"`        // example: Apache License 2.0
}
