package crd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func shouldSkipCrd(name string) bool {
	return !strings.Contains(name, "deckhouse.io")
}

func (l *CRDResources) crdsModuleRule(moduleName, path string) {
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
			return err
		}

		docs := splitManifests(string(fileContent))
		for _, d := range docs {
			var crd v1beta1.CustomResourceDefinition

			if err := yaml.Unmarshal([]byte(d), &crd); err != nil {
				errorList.Errorf("Can't parse manifests in %s folder: %s", rules.CrdsDir, err)

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
