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
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/promtool"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PrometheusRuleName = "prometheus-rules"
)

func NewPrometheusRule() *PrometheusRule {
	return &PrometheusRule{
		RuleMeta: pkg.RuleMeta{
			Name: PrometheusRuleName,
		},
	}
}

type PrometheusRule struct {
	pkg.RuleMeta
}

type checkResult struct {
	success bool
	errMsg  string
}

type rulesCacheStruct struct {
	cache map[string]checkResult
	mu    sync.RWMutex
}

var rulesCache = rulesCacheStruct{
	cache: make(map[string]checkResult),
	mu:    sync.RWMutex{},
}

func (*rulesCacheStruct) Put(hash string, value checkResult) {
	rulesCache.mu.Lock()
	defer rulesCache.mu.Unlock()

	rulesCache.cache[hash] = value
}

func (*rulesCacheStruct) Get(hash string) (checkResult, bool) {
	rulesCache.mu.RLock()
	defer rulesCache.mu.RUnlock()

	res, ok := rulesCache.cache[hash]
	return res, ok
}

type ModuleInterface interface {
	GetPath() string
}

func (r *PrometheusRule) ValidatePrometheusRules(m ModuleInterface, errorList *errors.LintRuleErrorsList) {
	modulePath := m.GetPath()
	errorList = errorList.WithFilePath(modulePath).WithRule(r.GetName())

	monitoringFilePath := filepath.Join(modulePath, "templates", "monitoring.yaml")
	if info, _ := os.Stat(monitoringFilePath); info == nil {
		errorList.WithFilePath(monitoringFilePath).
			Error("Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file")

		return
	}

	content, err := os.ReadFile(monitoringFilePath)
	if err != nil {
		errorList.WithFilePath(monitoringFilePath).
			Errorf("Cannot read 'templates/monitoring.yaml' file: %s", err)

		return
	}

	searchPath := filepath.Join(modulePath, "monitoring", "prometheus-rules")
	_, err = os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		errorList.Errorf("reading the 'monitoring/prometheus-rules' folder failed: %s", err)

		return
	}

	if isContentMatching(content, `include "helm_lib_prometheus_rules`) {
		return
	}

	desiredContent := `{{- include "helm_lib_prometheus_rules" (list . %q) }}`
	errorList.WithFilePath(monitoringFilePath).
		Errorf("The content of the 'templates/monitoring.yaml' should be equal to:\n%s",
			fmt.Sprintf(desiredContent, "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespace"),
		)
}

func isContentMatching(content []byte, desiredContent string) bool {
	foundIncludeLine := false
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		line = strings.ReplaceAll(line, " ", "")
		desiredContent = strings.ReplaceAll(desiredContent, " ", "")

		if strings.Contains(line, desiredContent) {
			foundIncludeLine = true
		}
	}

	if err := scanner.Err(); err != nil {
		return false
	}

	if foundIncludeLine {
		return true
	}

	return false
}

func (r *PrometheusRule) PromtoolRuleCheck(m ModuleInterface, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	if object.Unstructured.GetKind() != "PrometheusRule" {
		return
	}

	res, ok := rulesCache.Get(object.Hash)
	if ok {
		if !res.success {
			errorList.Errorf("Promtool check failed for Prometheus rule: %s", res.errMsg)
		}
		return
	}

	marshal, err := marshalStorageObject(object)
	if err != nil {
		errorList.Errorf("Error marshaling Prometheus rule to yaml: %s", err)
		return
	}

	err = promtool.CheckRules(marshal)
	if err != nil {
		rulesCache.Put(object.Hash, checkResult{
			success: false,
			errMsg:  err.Error(),
		})
		errorList.Errorf("Promtool check failed for Prometheus rule: %s", err.Error())
		return
	}

	rulesCache.Put(object.Hash, checkResult{success: true})
}

func marshalStorageObject(object storage.StoreObject) ([]byte, error) {
	ispec, ok := object.Unstructured.Object["spec"]
	if !ok {
		return nil, fmt.Errorf("spec field not found in object 'PrometheusRule'")
	}
	spec, ok := ispec.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("spec field is not a map[string]any")
	}
	marshal, err := yaml.Marshal(spec)
	if err != nil {
		return nil, err
	}
	return marshal, nil
}
