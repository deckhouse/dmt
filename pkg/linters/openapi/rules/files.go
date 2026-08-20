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
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
)

// openAPIAndCRDFiles returns the openapi/ files followed by the crds/ files.
// The two predicates are mutually exclusive (^openapi/ vs ^crds/), so this is a
// disjoint concatenation and keeps the order the linter used to feed the enum
// and high-availability rules in.
func openAPIAndCRDFiles(modulePath string) []string {
	return append(
		fsutils.GetFiles(modulePath, true, filterOpenAPIfiles),
		fsutils.GetFiles(modulePath, true, filterCRDsfiles)...)
}

// crdFiles returns only the crds/ files.
func crdFiles(modulePath string) []string {
	return fsutils.GetFiles(modulePath, true, filterCRDsfiles)
}

var openapiYamlRegex = regexp.MustCompile(`^openapi/.*\.ya?ml$`)

func filterOpenAPIfiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return openapiYamlRegex.MatchString(path)
}

var crdsYamlRegex = regexp.MustCompile(`^crds/.*\.ya?ml$`)

func filterCRDsfiles(rootPath, path string) bool {
	path = fsutils.Rel(rootPath, path)

	filename := filepath.Base(path)
	if strings.HasSuffix(filename, "-tests.yaml") {
		return false
	}

	if strings.HasPrefix(filename, "doc-ru-") {
		return false
	}

	return crdsYamlRegex.MatchString(path)
}

func bilingualCRDFiles(rootPath string) []string {
	crdsPath := filepath.Join(rootPath, "crds")

	entries, err := os.ReadDir(crdsPath)
	if err != nil {
		return nil
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
			continue
		}

		if strings.HasSuffix(filename, "-tests.yaml") {
			continue
		}

		result = append(result, filepath.Join(crdsPath, filename))
	}

	return result
}
