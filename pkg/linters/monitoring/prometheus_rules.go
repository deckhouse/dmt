/*
Copyright 2022 Flant JSC

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

package monitoring

import (
	"os"
	"os/exec"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
)

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

func (l *Monitoring) promtoolRuleCheck(m *module.Module, object storage.StoreObject) {
	errorList := l.ErrorList.WithModule(m.GetName()).WithFilePath(m.GetPath())

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
