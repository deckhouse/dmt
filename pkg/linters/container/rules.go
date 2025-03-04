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
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
)

func (l *Container) applyContainerRules(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		rules.NewRecommendedLabelsRule().ObjectRecommendedLabels,
		rules.NewNamespaceLabelsRule().ObjectNamespaceLabels,
		rules.NewAPIVersionRule().ObjectAPIVersion,
		rules.NewPriorityClassRule().ObjectPriorityClass,
		rules.NewDNSPolicyRule(l.cfg.ExcludeRules.DNSPolicy).
			ObjectDNSPolicy,
		rules.NewControllerSecurityContextRule(l.cfg.ExcludeRules.ControllerSecurityContext).
			ControllerSecurityContext,
		rules.NewRevisionHistoryLimitRule().ObjectRevisionHistoryLimit,
	}

	for _, rule := range objectRules {
		rule(object, errorList)
	}

	allContainers, err := object.GetAllContainers()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Cannot get containers from object: %s", err)

		return
	}

	if len(allContainers) == 0 {
		return
	}

	containerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		rules.NewNameDuplicatesRule().ContainerNameDuplicates,
		rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem).
			ObjectReadOnlyRootFilesystem,
		rules.NewHostNetworkPortsRule(l.cfg.ExcludeRules.HostNetworkPorts).ObjectHostNetworkPorts,

		// old with module names skipping
		rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates,
		rules.NewImageDigestRule(l.cfg.ExcludeRules.ImageDigest).ContainerImageDigestCheck,
		rules.NewImagePullPolicyRule().ContainersImagePullPolicy,
		rules.NewResourcesRule(l.cfg.ExcludeRules.Resources).
			ContainerStorageEphemeral,
		rules.NewContainerSecurityContextRule(l.cfg.ExcludeRules.SecurityContext).
			ContainerSecurityContext,
		rules.NewPortsRule(l.cfg.ExcludeRules.Ports).ContainerPorts,
	}

	for _, rule := range containerRules {
		rule(object, allContainers, errorList)
	}

	containers, err := object.GetContainers()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Cannot get containers from object: %s", err)

		return
	}

	if len(containers) == 0 {
		return
	}

	notInitContainerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		rules.NewLivenessRule(l.cfg.ExcludeRules.Liveness).
			CheckProbe,
		rules.NewReadinessRule(l.cfg.ExcludeRules.Readiness).
			CheckProbe,
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}

func (l *Container) shouldSkipModuleContainer(moduleName, container string) bool {
	for _, line := range l.cfg.SkipContainers {
		els := strings.Split(line, ":")
		if len(els) != 2 {
			continue
		}

		containerModuleName := strings.TrimSpace(els[0])
		containerName := strings.TrimSpace(els[1])

		checkContainer := container == containerName
		subString := strings.Trim(containerName, "*")
		if len(subString) != len(containerName) {
			checkContainer = strings.Contains(container, subString)
		}

		if moduleName == containerModuleName && checkContainer {
			return true
		}
	}

	return false
}
