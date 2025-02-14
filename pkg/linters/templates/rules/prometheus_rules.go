package rules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/module"
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

const promtoolPath = "/deckhouse/bin/promtool"

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

func marshalChartYaml(object storage.StoreObject) ([]byte, error) {
	marshal, err := yaml.Marshal(object.Unstructured.Object["spec"])
	if err != nil {
		return nil, err
	}
	return marshal, nil
}

func writeTempRuleFileFromObject(m *module.Module, marshalledYaml []byte) (string, error) {
	renderedFile, err := os.CreateTemp("", m.GetName()+".*.yml")
	if err != nil {
		return "", err
	}
	defer func(renderedFile *os.File) {
		_ = renderedFile.Close()
	}(renderedFile)

	_, err = renderedFile.Write(marshalledYaml)
	if err != nil {
		return "", err
	}
	_ = renderedFile.Sync()
	return renderedFile.Name(), nil
}

func checkRuleFile(path string) error {
	promtoolComand := exec.Command(promtoolPath, "check", "rules", path)
	_, err := promtoolComand.Output()
	return err
}

func (r *PrometheusRule) PromtoolRuleCheck(m *module.Module, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	// check promtoolPath exist, if not do not run linter
	if _, err := os.Stat(promtoolPath); err != nil {
		return
	}

	if object.Unstructured.GetKind() != "PrometheusRule" {
		return
	}

	res, ok := rulesCache.Get(object.Hash)
	if ok {
		if !res.success {
			errorList.Errorf("Promtool check failed for Helm chart: %s", res.errMsg)
		}
		return
	}

	marshal, err := marshalChartYaml(object)
	if err != nil {
		errorList.Error("Error marshaling Helm chart to yaml")
		return
	}

	path, err := writeTempRuleFileFromObject(m, marshal)
	defer os.Remove(path)

	if err != nil {
		errorList.Errorf("Error creating temporary rule file from Helm chart: %s", err.Error())
		return
	}

	err = checkRuleFile(path)
	if err != nil {
		errorMessage := string(err.(*exec.ExitError).Stderr)
		rulesCache.Put(object.Hash, checkResult{
			success: false,
			errMsg:  errorMessage,
		})
		errorList.Errorf("Promtool check failed for Helm chart: %s", errorMessage)
		return
	}

	rulesCache.Put(object.Hash, checkResult{success: true})
}

func (r *PrometheusRule) ValidatePrometheusRules(m *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(m.GetPath()).WithRule(r.GetName())

	monitoringFilePath := filepath.Join(m.GetPath(), "templates", "monitoring.yaml")
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

	searchPath := filepath.Join(m.GetPath(), "monitoring", "prometheus-rules")
	_, err = os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		errorList.Errorf("reading the 'monitoring/prometheus-rules' folder failed: %s", err)

		return
	}

	desiredContent := `{{- include "helm_lib_prometheus_rules" (list . %q) }}`

	if !isContentMatching(string(content), desiredContent, m.GetNamespace(), true) {
		errorList.WithFilePath(monitoringFilePath).
			Errorf("The content of the 'templates/monitoring.yaml' should be equal to:\n%s\nGot:\n%s",
				fmt.Sprintf(desiredContent, "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespaces"),
				string(content),
			)

		return
	}
}

func isContentMatching(content, desiredContent, moduleNamespace string, rulesEx bool) bool {
	for _, namespace := range []string{moduleNamespace, "d8-system", "d8-monitoring"} {
		checkContent := desiredContent
		if rulesEx {
			checkContent = fmt.Sprintf(desiredContent, namespace)
		}

		if strings.Contains(content, checkContent) {
			return true
		}
	}

	return false
}
