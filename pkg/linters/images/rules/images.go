package rules

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deckhouse/dmt/pkg/errors"
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

func skipModuleImageNameIfNeeded(filePath string) bool {
	for _, img := range Cfg.SkipModuleImageName {
		if strings.HasSuffix(filePath, img) {
			return true
		}
	}
	return false
}

func skipDistrolessImageCheckIfNeeded(image string) bool {
	for _, img := range Cfg.SkipDistrolessImageCheck {
		if strings.HasSuffix(image, img) {
			return true
		}
	}

	return false
}

func imageRegexp(s string) string {
	return fmt.Sprintf("^(from:|FROM)(\\s+)(%s)", s)
}

func isImageNameUnacceptable(imageName string) (bool, string) { //nolint:gocritic // false positive
	for ciVariable, pattern := range regexPatterns {
		matched, _ := regexp.MatchString(pattern, imageName)
		if matched {
			return true, ciVariable
		}
	}
	return false, ""
}

func checkImageNamesInDockerFiles(name, path string) *errors.LintRuleErrorsList {
	result := errors.NewError(ID, name)
	var filePaths []string
	imagesPath := filepath.Join(path, ImagesDir)

	if !IsExistsOnFilesystem(imagesPath) {
		return result
	}

	err := filepath.Walk(imagesPath, func(fullPath string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			return nil
		}

		if f.Name() == "Dockerfile" {
			filePaths = append(filePaths, fullPath)
		}

		return nil
	})
	if err != nil {
		return result.WithObjectID(imagesPath).Add(
			"Cannot read directory structure: %s",
			err.Error(),
		)
	}
	for _, path := range filePaths {
		if skipModuleImageNameIfNeeded(path) {
			continue
		}

		result.Merge(lintOneDockerfile(name, path, imagesPath))
	}

	return result
}

func lintOneDockerfile(name, path, imagesPath string) *errors.LintRuleErrorsList {
	result := errors.NewError(ID, name)
	relativeFilePath, err := filepath.Rel(imagesPath, path)
	if err != nil {
		return result.WithObjectID(path).Add(
			"Error calculating relative file path: %s",
			err.Error(),
		)
	}

	var (
		dockerfileFromInstructions []string
	)

	data, err := os.ReadFile(path)
	if err != nil {
		return result.WithObjectID(path).Add(
			"Error reading file: %s",
			err.Error(),
		)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	linePos := 0
	for scanner.Scan() {
		line := scanner.Text()
		linePos++
		ers, ciVariable := isImageNameUnacceptable(line)
		if ers {
			result.
				WithObjectID(fmt.Sprintf("module = %s, image = %s, line = %d", name, relativeFilePath, linePos)).
				Add(
					"Please use %s as an image name", ciVariable,
				)
		}

		if strings.HasPrefix(line, "FROM ") {
			dockerfileFromInstructions = append(dockerfileFromInstructions, strings.TrimPrefix(line, "FROM "))
		}
	}

	for i, fromInstruction := range dockerfileFromInstructions {
		if skipDistrolessImageCheckIfNeeded(relativeFilePath) {
			log.Printf("WARNING!!! SKIP DISTROLESS CHECK!!!\nmodule = %s, image = %s\nvalue - %s\n\n", name, relativeFilePath, fromInstruction)
			continue
		}

		ers, message := isDockerfileInstructionUnacceptable(fromInstruction, i == len(dockerfileFromInstructions)-1)
		if ers {
			result.WithObjectID(fmt.Sprintf("module = %s, path = %s", name, relativeFilePath)).
				WithValue(fromInstruction).
				Add("%s", message)
		}
	}

	return result
}

func isDockerfileInstructionUnacceptable(from string, final bool) (bool, string) { //nolint:gocritic // false positive
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
