package k8sresources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/images/rules"
)

var (
	sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")
)

func shouldSkipCrd(name string) bool {
	return !strings.Contains(name, "deckhouse.io")
}

func CrdsModuleRule(name, path string) *errors.LintRuleErrorsList {
	result := errors.NewError(rules.ID, name)
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
				result.WithObjectID("module = "+name).
					WithValue(err.Error()).
					Add("Can't parse manifests in %s folder", rules.CrdsDir)
				continue
			}

			if shouldSkipCrd(crd.Name) {
				continue
			}

			if crd.APIVersion != "apiextensions.k8s.io/v1" {
				result.WithObjectID(fmt.Sprintf("kind = %s ; name = %s ; module = %s ; file = %s", crd.Kind, crd.Name, name, path)).
					WithValue(crd.APIVersion).
					Add(`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`)
			}
		}
		return nil
	})
	return result
}

func splitManifests(bigFile string) []string {
	bigFileTmp := strings.TrimSpace(bigFile)
	return sep.Split(bigFileTmp, -1)
}
