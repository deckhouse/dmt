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

	"github.com/deckhouse/dmt/internal/fsutils"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	dockerfileRuleName = "dockerfile"
)

const (
	ImagesDir = "images"
)

func imageRegexp(s string) string {
	return fmt.Sprintf("^(from:|FROM)(\\s+)(%s)", s)
}

var regexPatterns = map[string]string{
	`$BASE_ALPINE`:           imageRegexp(`alpine:[\d.]+`),
	`$BASE_GOLANG_ALPINE`:    imageRegexp(`golang:1.15.[\d.]+-alpine3.12`),
	`$BASE_GOLANG_16_ALPINE`: imageRegexp(`golang:1.16.[\d.]+-alpine3.12`),
	`$BASE_GOLANG_BUSTER`:    imageRegexp(`golang:1.15.[\d.]+-buster`),
	`$BASE_GOLANG_16_BUSTER`: imageRegexp(`golang:1.16.[\d.]+-buster`),
	`$BASE_NGINX_ALPINE`:     imageRegexp(`nginx:[\d.]+-alpine`),
	`$BASE_PYTHON_ALPINE`:    imageRegexp(`python:[\d.]+-alpine`),
	`$BASE_UBUNTU`:           imageRegexp(`ubuntu:[\d.]+`),
	`$BASE_JEKYLL`:           imageRegexp(`jekyll/jekyll:[\d.]+`),
	`$BASE_SCRATCH`:          imageRegexp(`scratch:[\d.]+`),
}

type ImageRule struct {
	pkg.RuleMeta
	pkg.PrefixRule
}

func NewImageRule(cfg *config.ImageSettings) *ImageRule {
	return &ImageRule{
		RuleMeta: pkg.RuleMeta{
			Name: dockerfileRuleName,
		},
		PrefixRule: pkg.PrefixRule{
			ExcludeRules: cfg.ExcludeRules.SkipImageFilePathPrefix.Get(),
		},
	}
}

func isImageNameUnacceptable(imageName string) (bool, string) {
	for ciVariable, pattern := range regexPatterns {
		matched, _ := regexp.MatchString(pattern, imageName)
		if matched {
			return true, ciVariable
		}
	}
	return false, ""
}

func (r *ImageRule) CheckImageNamesInDockerFiles(modulePath string, errorList *errors.LintRuleErrorsList) {
	imagesPath := filepath.Join(modulePath, ImagesDir)
	if !fsutils.IsFileExist(imagesPath) {
		return
	}

	filePaths := fsutils.GetFiles(imagesPath, false, func(_, path string) bool {
		return filepath.Base(path) == "Dockerfile"
	})

	for _, path := range filePaths {
		if !r.PrefixRule.Enabled(path) {
			continue
		}
		r.lintOneDockerfile(path, imagesPath, errorList)
	}
}

func (*ImageRule) lintOneDockerfile(path, imagesPath string, errorList *errors.LintRuleErrorsList) {
	relativeFilePath := fsutils.Rel(imagesPath, path)
	errorList = errorList.WithFilePath(relativeFilePath).WithRule(dockerfileRuleName)

	data, err := os.ReadFile(path)
	if err != nil {
		errorList.WithFilePath(path).
			Errorf("Error reading file: %s", err)

		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	linePos := 0
	for scanner.Scan() {
		line := scanner.Text()
		linePos++
		ers, ciVariable := isImageNameUnacceptable(line)
		if ers {
			errorList.WithObjectID(fmt.Sprintf("image = %s", relativeFilePath)).
				WithLineNumber(linePos).
				Errorf("Please use %s as an image name", ciVariable)
		}
	}
}
