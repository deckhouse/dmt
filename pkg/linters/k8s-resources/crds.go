package k8sresources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"
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
	var lintRuleErrorsList *errors.LintRuleErrorsList
	_ = filepath.Walk(path, func(path string, _ os.FileInfo, _ error) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		fileContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		bigFileTmp := strings.TrimSpace(string(fileContent))
		docs := sep.Split(bigFileTmp, -1)
		for _, d := range docs {
			if d == "" {
				continue
			}

			d = strings.TrimSpace(d)
			var crd v1beta1.CustomResourceDefinition

			err = yaml.Unmarshal([]byte(d), &crd)
			if err != nil {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					rules.ID,
					"module = "+name,
					err.Error(),
					"Can't parse manifests in %s folder", rules.CrdsDir,
				))
			}

			if shouldSkipCrd(crd.Name) {
				continue
			}

			if crd.APIVersion != "apiextensions.k8s.io/v1" {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					rules.ID,
					d,
					fmt.Sprintf("kind = %s ; name = %s ; module = %s ; file = %s", crd.Kind, crd.Name, name, path),
					crd.APIVersion,
					`CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"`,
				))
			}
		}
		return nil
	})
	return lintRuleErrorsList
}
