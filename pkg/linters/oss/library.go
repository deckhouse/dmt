package oss

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/pkg/errors"
)

const ossFilename = "oss.yaml"

func (o *OSS) ossModuleRule(name, moduleRoot string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("oss", name)

	if errs := o.verifyOssFile(name, moduleRoot); len(errs) > 0 {
		for _, err := range errs {
			result.WithObjectID(moduleRoot).Add("%v", ossFileErrorMessage(err))
		}
	}

	return result
}

func ossFileErrorMessage(err error) string {
	if os.IsNotExist(err) {
		return "Module should have " + ossFilename
	}
	return fmt.Sprintf("Invalid %s: %s", ossFilename, err.Error())
}

func (o *OSS) verifyOssFile(name, moduleRoot string) []error {
	if o.shouldIgnoreOssInfo(name) {
		return nil
	}

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

// TODO When lintignore files will be implemented in helm, detect "oss.yaml" line in it
func (o *OSS) shouldIgnoreOssInfo(moduleName string) bool {
	return slices.Contains(o.cfg.SkipOssChecks, moduleName)
}

type ossProject struct {
	Name        string `yaml:"name"`           // example: Dex
	Description string `yaml:"description"`    // example: A Federated OpenID Connect Provider with pluggable connectors
	Link        string `yaml:"link"`           // example: https://github.com/dexidp/dex
	Logo        string `yaml:"logo,omitempty"` // example: https://dexidp.io/img/logos/dex-horizontal-color.png
	License     string `yaml:"license"`        // example: Apache License 2.0
}
