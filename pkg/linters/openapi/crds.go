package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

const CrdsDir = "crds"

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func shouldSkipCrd(name string) bool {
	return !strings.Contains(name, "deckhouse.io")
}

type CRDValidator struct {
	cfg       *config.CRDResourcesSettings
	ErrorList *errors.LintRuleErrorsList
}

func NewCRDValidator(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *CRDValidator {
	return &CRDValidator{
		cfg:       &cfg.LintersSettings.CRDResources,
		ErrorList: errorList.WithLinterID("crd").WithMaxLevel(cfg.LintersSettings.CRDResources.Impact),
	}
}

func (l *CRDValidator) Run(moduleName, path string) {
	if !isExistsOnFilesystem(moduleName, path) {
		return
	}

	errorList := l.ErrorList.WithModule(moduleName)

	_ = filepath.Walk(path, func(path string, _ os.FileInfo, _ error) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		fileContent, err := os.ReadFile(path)
		if err != nil {
			errorList.Errorf("Can't read file %s: %s", path, err)
			return err
		}

		docs := splitManifests(string(fileContent))
		for _, d := range docs {
			var crd v1beta1.CustomResourceDefinition

			if err := yaml.Unmarshal([]byte(d), &crd); err != nil {
				errorList.Errorf("Can't parse manifests in %s folder: %s", CrdsDir, err)

				continue
			}

			if shouldSkipCrd(crd.Name) {
				continue
			}

			if crd.APIVersion != "apiextensions.k8s.io/v1" {
				errorList.WithObjectID(fmt.Sprintf("kind = %s ; name = %s ; module = %s ; file = %s", crd.Kind, crd.Name, moduleName, path)).
					WithValue(crd.APIVersion).
					Errorf(`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`)
			}
		}

		return nil
	})
}

func splitManifests(bigFile string) []string {
	bigFileTmp := strings.TrimSpace(bigFile)
	return sep.Split(bigFileTmp, -1)
}

func isExistsOnFilesystem(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
