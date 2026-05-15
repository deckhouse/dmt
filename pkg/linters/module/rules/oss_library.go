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

	"github.com/Masterminds/semver/v3"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	OSSRuleName = "oss"
)

// NewOSSRule creates an OSS attribution rule instance.
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

const (
	ossFilename = "oss.yaml"
	imagesDir   = "images"
)

// OssModuleRule validates oss.yaml only for modules that contain image build sources.
func (r *OSSRule) OssModuleRule(moduleRoot string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(filepath.Join(moduleRoot, ossFilename))

	if !r.Enabled() {
		errorList = errorList.WithMaxLevel(ptr.To(pkg.Ignored))
	}

	imagesPath := filepath.Join(moduleRoot, imagesDir)

	info, err := os.Stat(imagesPath)
	if err != nil || !info.IsDir() {
		return
	}

	verifyOssFile(moduleRoot, errorList)
}

// ossFileErrorMessage formats user-facing errors for missing or invalid oss.yaml.
func ossFileErrorMessage(err error) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("module has %s folder, so it likely should have %s", imagesDir, ossFilename)
	}

	return fmt.Sprintf("invalid %s: %s", ossFilename, err.Error())
}

// verifyOssFile reads oss.yaml and validates every described OSS project.
func verifyOssFile(moduleRoot string, errorList *errors.LintRuleErrorsList) {
	projects, err := readOssFile(moduleRoot)
	if err != nil {
		if os.IsNotExist(err) {
			errorList.Warn(ossFileErrorMessage(err))
		} else {
			errorList.Error(ossFileErrorMessage(err))
		}

		return
	}

	if len(projects) == 0 {
		errorList.Error("no projects described")

		return
	}

	projectIDs := make(map[string]int, len(projects))

	for i, p := range projects {
		if projectID := strings.TrimSpace(p.ID); projectID != "" {
			if prevIndex, ok := projectIDs[projectID]; ok {
				prefix := fmt.Sprintf("#%d (id=%s)", i+1, projectID)
				errorList.WithObjectID("index="+prefix+";").
					Errorf("id must be unique; duplicate id %q already used by project #%d", projectID, prevIndex)
			} else {
				projectIDs[projectID] = i + 1
			}
		}

		assertOssProject(i+1, &p, errorList)
	}
}

// assertOssProject validates required attribution fields for one OSS project.
func assertOssProject(i int, p *ossProject, errorList *errors.LintRuleErrorsList) {
	// prefix to make it easier navigate among errors
	prefix := fmt.Sprintf("#%d", i)

	// ID
	if strings.TrimSpace(p.ID) == "" {
		errorList.WithObjectID("index=" + prefix + ";").Error("id must not be empty")
	} else {
		prefix = fmt.Sprintf("#%d (id=%s)", i, p.ID)
	}

	assertOssProjectVersion(prefix, p, errorList)

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

// assertOssProjectVersion validates simple and conditional version definitions.
func assertOssProjectVersion(prefix string, p *ossProject, errorList *errors.LintRuleErrorsList) {
	version := strings.TrimSpace(p.Version)
	hasVersions := len(p.Versions) > 0

	if version == "" && !hasVersions {
		errorList.WithObjectID("index=" + prefix + ";").
			Error("version must not be empty. Please fill in the parameter and configure CI (werf files for module images) to use these setting.")

		return
	}

	if version != "" {
		assertOssVersionValue(prefix, "version", version, errorList)
	}

	if version != "" && hasVersions {
		errorList.WithObjectID("index=" + prefix + ";").Error("version and versions must not be used together")
	}

	for i, versionItem := range p.Versions {
		versionPrefix := fmt.Sprintf("%s.versions[%d]", prefix, i)

		versionValue := strings.TrimSpace(versionItem.Version)
		if versionValue == "" {
			errorList.WithObjectID("index=" + versionPrefix + ";").
				Error("versions[].version must not be empty. Please fill in the parameter and configure CI (werf files for module images) to use these setting.")

			continue
		}

		assertOssVersionValue(versionPrefix, "versions[].version", versionValue, errorList)
	}
}

// assertOssVersionValue warns when a non-empty version is not semver-compatible.
func assertOssVersionValue(prefix, fieldPath, version string, errorList *errors.LintRuleErrorsList) {
	if _, err := semver.NewVersion(version); err != nil {
		errorList.WithObjectID("index=" + prefix + ";").
			Warn(fmt.Sprintf("%s %q is not semver-compatible; if this version is correct for the OSS project, ignore this warning: %v", fieldPath, version, err))
	}
}

// readOssFile loads oss.yaml from the module root and parses its project list.
func readOssFile(moduleRoot string) ([]ossProject, error) {
	b, err := os.ReadFile(filepath.Join(moduleRoot, ossFilename))
	if err != nil {
		return nil, err
	}

	return parseProjectList(b)
}

// parseProjectList parses oss.yaml content using strict YAML decoding.
func parseProjectList(b []byte) ([]ossProject, error) {
	var projects []ossProject

	err := yaml.UnmarshalStrict(b, &projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

type ossProject struct {
	Name        string       `json:"name"`           // example: Dex
	Description string       `json:"description"`    // example: A Federated OpenID Connect Provider with pluggable connectors
	Link        string       `json:"link"`           // example: https://github.com/dexidp/dex
	Logo        string       `json:"logo,omitempty"` // example: https://dexidp.io/img/logos/dex-horizontal-color.png
	License     string       `json:"license"`        // example: Apache License 2.0
	ID          string       `json:"id"`             // example: dexidp/dex
	Version     string       `json:"version"`        // example: 2.0.0
	Versions    []ossVersion `json:"versions,omitempty"`
}

type ossVersion struct {
	Condition map[string]any `json:"condition,omitempty"`
	Name      string         `json:"name,omitempty"`
	Version   string         `json:"version"`
}
