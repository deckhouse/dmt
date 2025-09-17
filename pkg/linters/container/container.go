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

package container

import (
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "container"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *pkg.ContainerLinterConfig
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Container {
	containerCfg := convertToContainerLinterConfig(&cfg.LintersSettings.Container)
	return &Container{
		name:      ID,
		desc:      "Lint container objects",
		cfg:       containerCfg,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(containerCfg.Impact),
	}
}

func convertToContainerLinterConfig(oldCfg *config.ContainerSettings) *pkg.ContainerLinterConfig {
	newCfg := &pkg.ContainerLinterConfig{}
	newCfg.Impact = oldCfg.Impact

	newCfg.Rules.RecommendedLabelsRule.SetLevel(oldCfg.RecommendedLabelsRule.Impact)

	newCfg.ExcludeRules = pkg.ContainerExcludeRules{
		ControllerSecurityContext: convertKindRuleExcludeList(oldCfg.ExcludeRules.ControllerSecurityContext),
		DNSPolicy:                 convertKindRuleExcludeList(oldCfg.ExcludeRules.DNSPolicy),
		HostNetworkPorts:          convertContainerRuleExcludeList(oldCfg.ExcludeRules.HostNetworkPorts),
		Ports:                     convertContainerRuleExcludeList(oldCfg.ExcludeRules.Ports),
		ReadOnlyRootFilesystem:    convertContainerRuleExcludeList(oldCfg.ExcludeRules.ReadOnlyRootFilesystem),
		ImageDigest:               convertContainerRuleExcludeList(oldCfg.ExcludeRules.ImageDigest),
		Resources:                 convertContainerRuleExcludeList(oldCfg.ExcludeRules.Resources),
		SecurityContext:           convertContainerRuleExcludeList(oldCfg.ExcludeRules.SecurityContext),
		Liveness:                  convertContainerRuleExcludeList(oldCfg.ExcludeRules.Liveness),
		Readiness:                 convertContainerRuleExcludeList(oldCfg.ExcludeRules.Readiness),
		Description:               convertStringRuleExcludeList(oldCfg.ExcludeRules.Description),
	}

	return newCfg
}

func convertKindRuleExcludeList(oldList config.KindRuleExcludeList) pkg.KindRuleExcludeList {
	newList := make(pkg.KindRuleExcludeList, len(oldList))
	for i, item := range oldList {
		newList[i] = pkg.KindRuleExclude{Kind: item.Kind, Name: item.Name}
	}
	return newList
}

func convertContainerRuleExcludeList(oldList config.ContainerRuleExcludeList) pkg.ContainerRuleExcludeList {
	newList := make(pkg.ContainerRuleExcludeList, len(oldList))
	for i, item := range oldList {
		newList[i] = pkg.ContainerRuleExclude{Kind: item.Kind, Name: item.Name, Container: item.Container}
	}
	return newList
}

func convertStringRuleExcludeList(oldList config.StringRuleExcludeList) pkg.StringRuleExcludeList {
	newList := make(pkg.StringRuleExcludeList, len(oldList))
	for i, item := range oldList {
		newList[i] = string(item)
	}
	return newList
}

func (l *Container) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	for _, object := range m.GetStorage() {
		l.applyContainerRules(object, errorList)
	}
}

func (l *Container) Name() string {
	return l.name
}

func (l *Container) Desc() string {
	return l.desc
}
