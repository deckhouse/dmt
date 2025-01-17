package rules

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/internal/werf"
	"github.com/deckhouse/dmt/pkg/errors"
)

type werfFile struct {
	Artifact string `json:"artifact" yaml:"artifact"`
	Image    string `json:"image" yaml:"image"`
	From     string `json:"from" yaml:"from"`
	Final    *bool  `json:"final" yaml:"final"`
}

func lintWerfFile(moduleName, path string) *errors.LintRuleErrorsList {
	result := &errors.LintRuleErrorsList{}
	data, err := werf.GetWerfConfig(path)
	if err != nil {
		return result.Add(
			errors.NewLintRuleError(
				ID,
				path,
				moduleName,
				path,
				"Error reading werf file: %s",
				err.Error(),
			))
	}

	werfDocs := splitManifests(data)

	for _, doc := range werfDocs {
		var w werfFile
		err = yaml.Unmarshal([]byte(doc), &w)
		if err != nil {
			// skip invalid yaml documents
			continue
		}

		w.From = strings.TrimSpace(w.From)
		if w.From == "" {
			continue
		}

		if w.Artifact != "" {
			result.Add(
				errors.NewLintRuleError(
					ID,
					path,
					moduleName,
					w.From,
					"Use `from:` or `fromImage:` and `final: false` directives instead of `artifact:` in the werf file",
				),
			)
			continue
		}

		if w.Final != nil && !*w.Final {
			// skip image, if it's not final
			continue
		}

		// TODO: add skips for some images

		if !isWerfImagesCorrect(w.From) {
			result.Add(
				errors.NewLintRuleError(
					ID,
					path,
					moduleName,
					w.From,
					"`from:` parameter should be one of our BASE_DISTROLESS images",
				),
			)
			continue
		}
	}

	return result
}

func splitManifests(bigFile string) map[string]string {
	var sep = regexp.MustCompile("(?:^|\\s*\n)---\\s*")

	tpl := "manifest-%d"
	res := map[string]string{}
	// Making sure that any extra whitespace in YAML stream doesn't interfere in splitting documents correctly.
	bigFileTmp := strings.TrimSpace(bigFile)
	docs := sep.Split(bigFileTmp, -1)
	var count int
	for _, d := range docs {
		if d == "" {
			continue
		}

		d = strings.TrimSpace(d)
		res[fmt.Sprintf(tpl, count)] = d
		count++
	}
	return res
}

func isWerfImagesCorrect(img string) bool {
	s := strings.Split(img, "/")
	if len(s) < 2 {
		return false
	}
	if s[1] != "base_images" {
		return false
	}

	return true
}
