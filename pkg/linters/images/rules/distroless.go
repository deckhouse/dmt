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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	distrolessRuleName = "distroless"
)

var distrolessImagesPrefix = map[string][]string{
	"docker": {
		"$BASE_DISTROLESS",
		"$BASE_ALT",
	},
}

type DistrolessRule struct {
	pkg.RuleMeta
	SkipDistrolessFilePathPrefix pkg.PrefixRule
}

func NewDistrolessRule(cfg *config.ImageSettings) *DistrolessRule {
	return &DistrolessRule{
		RuleMeta: pkg.RuleMeta{
			Name: distrolessRuleName,
		},
		SkipDistrolessFilePathPrefix: pkg.PrefixRule{
			ExcludeRules: cfg.ExcludeRules.SkipDistrolessFilePathPrefix.Get(),
		},
	}
}

func (r *DistrolessRule) CheckImageNamesInDockerFiles(modulePath string, errorList *errors.LintRuleErrorsList) {
	imagesPath := filepath.Join(modulePath, ImagesDir)
	if !fsutils.IsFileExist(imagesPath) {
		return
	}

	filePaths := fsutils.GetFiles(imagesPath, false, func(_, path string) bool {
		return filepath.Base(path) == "Dockerfile"
	})

	for _, path := range filePaths {
		r.lintOneDockerfile(path, imagesPath, errorList)
	}
}

func (r *DistrolessRule) lintOneDockerfile(path, imagesPath string, errorList *errors.LintRuleErrorsList) {
	relativeFilePath := fsutils.Rel(imagesPath, path)
	errorList = errorList.WithFilePath(relativeFilePath).WithRule(distrolessRuleName)

	var (
		dockerfileFromInstructions []string
	)

	data, err := os.ReadFile(path)
	if err != nil {
		errorList.WithFilePath(path).
			Errorf("Error reading file: %s", err)

		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "FROM ") {
			dockerfileFromInstructions = append(dockerfileFromInstructions, strings.TrimPrefix(line, "FROM "))
		}
	}

	for i, fromInstruction := range dockerfileFromInstructions {
		if !r.SkipDistrolessFilePathPrefix.Enabled(relativeFilePath) {
			errorList.WithObjectID(fmt.Sprintf("image = %s ; value - %s", relativeFilePath, fromInstruction)).
				Warn("WARNING!!! SKIP DISTROLESS CHECK!!!")

			continue
		}

		ers, message := isDockerfileInstructionUnacceptable(fromInstruction, i == len(dockerfileFromInstructions)-1)
		if ers {
			errorList.WithFilePath(relativeFilePath).
				WithValue(fromInstruction).
				Error(message)
		}
	}
}

func isDockerfileInstructionUnacceptable(from string, final bool) (bool, string) {
	if from == "scratch" {
		return false, ""
	}

	if final {
		if !checkDistrolessPrefix(from, distrolessImagesPrefix["docker"]) {
			return true, "Last `FROM` instruction should use one of our $BASE_DISTROLESS images"
		}
	} else {
		matched, _ := regexp.MatchString("@sha256:[A-Fa-f0-9]{64}", from)
		if !strings.HasPrefix(from, "$BASE_") && !matched {
			return true, "Intermediate `FROM` instructions should use one of our $BASE_ images or have `@sha526:` checksum specified"
		}
	}

	return false, ""
}

func checkDistrolessPrefix(str string, in []string) bool {
	str = strings.TrimPrefix(str, "$.Images.")
	str = strings.TrimPrefix(str, ".Images.")

	for _, pattern := range in {
		if strings.HasPrefix(str, pattern) {
			return true
		}
	}

	return false
}
