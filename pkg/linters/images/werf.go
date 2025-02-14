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

package images

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	werfRuleName = "werf"
)

type werfFile struct {
	Artifact string `json:"artifact" yaml:"artifact"`
	Image    string `json:"image" yaml:"image"`
	From     string `json:"from" yaml:"from"`
	Final    *bool  `json:"final" yaml:"final"`
}

func lintWerfFile(data string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(werfRuleName)
	werfDocs := splitManifests(data)

	i := 1
	for _, doc := range werfDocs {
		var w werfFile
		err := yaml.Unmarshal([]byte(doc), &w)
		if err != nil {
			// skip invalid yaml documents
			continue
		}

		w.From = strings.TrimSpace(w.From)
		if w.From == "" {
			continue
		}

		if w.Artifact != "" {
			errorList.WithObjectID("werf.yaml:manifest-" + strconv.Itoa(i)).
				WithValue("artifact: " + w.Artifact).
				Error("Use `from:` or `fromImage:` and `final: false` directives instead of `artifact:` in the werf file")
		}

		if w.Final != nil && !*w.Final {
			// skip image, if it's not final
			continue
		}

		// TODO: add skips for some images

		if !isWerfImagesCorrect(w.From) {
			errorList.WithObjectID("werf.yaml:manifest-" + strconv.Itoa(i)).
				WithValue("from: " + w.From).
				Error("`from:` parameter should be one of our BASE_DISTROLESS images")
		}
		i++
	}
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
