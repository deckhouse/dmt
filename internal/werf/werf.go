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

package werf

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/dmt/internal/fsutils"
)

const (
	werfFileName = "werf.yaml"
)

var defaultWerfConfigTemplatesDirName = ".werf"

func GetWerfConfig(dir string) (string, error) {
	werfFile := getRootWerfFile(dir)
	if werfFile == "" {
		return "", nil
	}

	tmpl := template.New("werfConfig")
	tmpl.Funcs(funcMap(tmpl))

	{
		content, err := os.ReadFile(werfFile)
		if err != nil {
			return "", err
		}
		tmpl, err = tmpl.Parse(string(content))
		if err != nil {
			return "", err
		}
	}

	if err := parseWerfConfigTemplatesDir(filepath.Dir(werfFile), tmpl); err != nil {
		return "", err
	}

	templateData := make(map[string]any)
	templateData["Files"] = NewFiles(werfFile, dir)
	templateData["Env"] = cmp.Or(os.Getenv("WERF_ENV"), "EE")

	templateData["Commit"] = map[string]any{
		"Hash": "hash",
		"Date": map[string]string{
			"Human": time.Now().Format(time.RFC3339),
			"Unix":  strconv.FormatInt(time.Now().Unix(), 10),
		},
	}

	return executeTemplate(tmpl, "werfConfig", templateData)
}

func getRootWerfFile(dir string) string {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		log.Warn("Can't make abs path", slog.String("path", dir), log.Err(err))
		absPath = filepath.Clean(dir)
	}

	// start from it
	var currentDir string
	if fsutils.IsDir(absPath) {
		currentDir = absPath
	} else {
		currentDir = filepath.Dir(absPath)
	}

	for {
		result := filepath.Join(currentDir, werfFileName)
		if fsutils.IsFile(result) {
			return result
		}
		currentDir = filepath.Dir(currentDir)
		if currentDir == "/" {
			break
		}
	}

	return ""
}

func parseWerfConfigTemplatesDir(rootDir string, tmpl *template.Template) error {
	templatesDir := filepath.Join(rootDir, defaultWerfConfigTemplatesDirName)
	if !fsutils.IsDir(templatesDir) {
		return nil
	}

	if err := filepath.WalkDir(templatesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".tmpl" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(path[len(templatesDir)+1:])
			if err := addTemplate(tmpl, name, string(data)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func addTemplate(tmpl *template.Template, templateName, templateContent string) error {
	extraTemplate := tmpl.New(templateName)
	_, err := extraTemplate.Parse(templateContent)
	return err
}

func executeTemplate(tmpl *template.Template, name string, data any) (string, error) {
	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

func funcMap(tmpl *template.Template) template.FuncMap {
	funcMap := sprig.TxtFuncMap()
	delete(funcMap, "expandenv")

	funcMap["fromYaml"] = func(str string) (map[string]any, error) {
		m := map[string]any{}

		if err := yaml.Unmarshal([]byte(str), &m); err != nil {
			return nil, err
		}

		return m, nil
	}
	funcMap["include"] = func(name string, data any) (string, error) {
		return executeTemplate(tmpl, name, data)
	}
	funcMap["tpl"] = func(templateContent string, data any) (string, error) {
		templateName := generateRandomTemplateFuncName()
		if err := addTemplate(tmpl, templateName, templateContent); err != nil {
			return "", err
		}

		return executeTemplate(tmpl, templateName, data)
	}

	funcMap["env"] = func(value any, args ...string) (string, error) {
		if len(args) > 1 {
			return "", fmt.Errorf("more than 1 optional argument prohibited")
		}

		envVarName := fmt.Sprint(value)

		var fallbackValue *string
		if len(args) == 1 {
			fallbackValue = &args[0]
		}

		envVarValue, envVarFound := os.LookupEnv(envVarName)
		if !envVarFound {
			if fallbackValue != nil {
				return *fallbackValue, nil
			}
			return "", nil
		}

		if envVarValue == "" && fallbackValue != nil {
			return *fallbackValue, nil
		}

		return envVarValue, nil
	}

	funcMap["required"] = func(msg string, val any) (any, error) {
		switch val {
		case nil:
			return nil, errors.New(msg)
		case "":
			return val, errors.New(msg)
		}
		return val, nil
	}

	return funcMap
}

func generateRandomTemplateFuncName() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const templateFuncNameLength = 10

	b := make([]byte, templateFuncNameLength)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		b[i] = letterBytes[n.Int64()]
	}

	return string(b)
}
