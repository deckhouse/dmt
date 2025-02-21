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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	dockerfileRuleName = "dockerfile"
)

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

var distrolessImagesPrefix = map[string][]string{
	"docker": {
		"$BASE_DISTROLESS",
		"$BASE_ALT",
	},
}

func (l *Images) skipModuleImageNameIfNeeded(filePath string) bool {
	for _, img := range l.cfg.SkipModuleImageName {
		if strings.HasSuffix(filePath, img) {
			return true
		}
	}

	return false
}

func (l *Images) skipDistrolessImageCheckIfNeeded(image string) bool {
	for _, img := range l.cfg.SkipDistrolessImageCheck {
		if strings.HasSuffix(image, img) {
			return true
		}
	}

	return false
}

func imageRegexp(s string) string {
	return fmt.Sprintf("^(from:|FROM)(\\s+)(%s)", s)
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

func (l *Images) checkImageNamesInDockerFiles(moduleName, modulePath string, errorList *errors.LintRuleErrorsList) {
	var filePaths []string

	imagesPath := filepath.Join(modulePath, ImagesDir)

	if !IsExistsOnFilesystem(imagesPath) {
		return
	}

	_ = filepath.Walk(imagesPath, func(fullPath string, f os.FileInfo, _ error) error {
		if f.IsDir() {
			return nil
		}

		if f.Name() == "Dockerfile" {
			filePaths = append(filePaths, fullPath)
		}

		return nil
	})

	for _, path := range filePaths {
		if l.skipModuleImageNameIfNeeded(path) {
			continue
		}

		l.lintOneDockerfile(moduleName, path, imagesPath, errorList)
	}
}

func (l *Images) lintOneDockerfile(moduleName, path, imagesPath string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(path).WithRule(dockerfileRuleName)
	relativeFilePath, err := filepath.Rel(imagesPath, path)
	if err != nil {
		errorList.WithFilePath(path).
			Errorf("Error calculating relative file path: %s", err)

		return
	}

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

		if strings.HasPrefix(line, "FROM ") {
			dockerfileFromInstructions = append(dockerfileFromInstructions, strings.TrimPrefix(line, "FROM "))
		}
	}

	for i, fromInstruction := range dockerfileFromInstructions {
		if l.skipDistrolessImageCheckIfNeeded(relativeFilePath) {
			errorList.WithObjectID(fmt.Sprintf("module = %s ; image = %s ; value - %s", moduleName, relativeFilePath, fromInstruction)).
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
	result := false

	str = strings.TrimPrefix(str, "$.Images.")
	str = strings.TrimPrefix(str, ".Images.")

	for _, pattern := range in {
		if strings.HasPrefix(str, pattern) {
			result = true

			break
		}
	}

	return result
}
