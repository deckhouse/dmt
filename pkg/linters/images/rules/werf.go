package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/werf"
	"github.com/deckhouse/dmt/pkg/errors"
	"gopkg.in/yaml.v3"
)

func lintWerfFile(moduleName, path string) (errLint *errors.LintRuleErrorsList) {
	errLint = &errors.LintRuleErrorsList{}
	data, err := werf.GetWerfConfig(path)
	if err != nil {
		return errLint.Add(
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
			errLint.Add(
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

		// if skipDistrolessImageCheckIfNeeded(relativeFilePath) {
		// 	log.Printf("WARNING!!! SKIP DISTROLESS CHECK!!!\nmodule = %s, image = %s\nvalue - %s\n\n", moduleName, relativeFilePath, w.From)
		// 	return nil
		// }

		if !checkDistrolessPrefix(w.From, distrolessImagesPrefix["werf"]) {
			errLint.Add(
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

	return errLint
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

type werfFile struct {
	Artifact string `json:"artifact" yaml:"artifact"`
	Image    string `json:"image" yaml:"image"`
	From     string `json:"from" yaml:"from"`
	Final    *bool  `json:"final" yaml:"final"`
}
