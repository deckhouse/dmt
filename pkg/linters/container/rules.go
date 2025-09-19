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
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
)

func (l *Container) applyContainerRules(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.GetPath())

	rules.NewRecommendedLabelsRule().ObjectRecommendedLabels(object, errorList.WithRule("recommended-labels").WithMaxLevel(l.cfg.Rules.RecommendedLabelsRule.GetLevel()))
	rules.NewNamespaceLabelsRule().ObjectNamespaceLabels(object, errorList)
	rules.NewAPIVersionRule().ObjectAPIVersion(object, errorList)
	rules.NewPriorityClassRule().ObjectPriorityClass(object, errorList)
	rules.NewDNSPolicyRule(l.cfg.ExcludeRules.DNSPolicy.Get()).
		ObjectDNSPolicy(object, errorList)
	rules.NewControllerSecurityContextRule(l.cfg.ExcludeRules.ControllerSecurityContext.Get()).
		ControllerSecurityContext(object, errorList)
	rules.NewRevisionHistoryLimitRule().ObjectRevisionHistoryLimit(object, errorList)

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
		rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get()).
			ObjectReadOnlyRootFilesystem,
		rules.NewNoNewPrivilegesRule(l.cfg.ExcludeRules.NoNewPrivileges.Get()).
			ContainerNoNewPrivileges,
		rules.NewSeccompProfileRule(l.cfg.ExcludeRules.SeccompProfile.Get()).
			ContainerSeccompProfile,
		rules.NewHostNetworkPortsRule(l.cfg.ExcludeRules.HostNetworkPorts.Get()).ObjectHostNetworkPorts,

		// old with module names skipping
		rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates,
		rules.NewImageDigestRule(l.cfg.ExcludeRules.ImageDigest.Get()).ContainerImageDigestCheck,
		rules.NewImagePullPolicyRule().ContainersImagePullPolicy,
		rules.NewResourcesRule(l.cfg.ExcludeRules.Resources.Get()).
			ContainerStorageEphemeral,
		rules.NewContainerSecurityContextRule(l.cfg.ExcludeRules.SecurityContext.Get()).
			ContainerSecurityContext,
		rules.NewPortsRule(l.cfg.ExcludeRules.Ports.Get()).ContainerPorts,
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
		rules.NewLivenessRule(l.cfg.ExcludeRules.Liveness.Get()).
			CheckProbe,
		rules.NewReadinessRule(l.cfg.ExcludeRules.Readiness.Get()).
			CheckProbe,
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}
