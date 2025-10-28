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

	rules.NewRecommendedLabelsRule().ObjectRecommendedLabels(object, errorList.WithMaxLevel(l.cfg.Rules.RecommendedLabelsRule.GetLevel()))
	rules.NewNamespaceLabelsRule().ObjectNamespaceLabels(object, errorList.WithMaxLevel(l.cfg.Rules.NamespaceLabelsRule.GetLevel()))
	rules.NewAPIVersionRule().ObjectAPIVersion(object, errorList.WithMaxLevel(l.cfg.Rules.ApiVersionRule.GetLevel()))
	rules.NewPriorityClassRule().ObjectPriorityClass(object, errorList.WithMaxLevel(l.cfg.Rules.PriorityClassRule.GetLevel()))
	rules.NewDNSPolicyRule(l.cfg.ExcludeRules.DNSPolicy.Get()).
		ObjectDNSPolicy(object, errorList.WithMaxLevel(l.cfg.Rules.DNSPolicyRule.GetLevel()))
	rules.NewControllerSecurityContextRule(l.cfg.ExcludeRules.ControllerSecurityContext.Get()).
		ControllerSecurityContext(object, errorList.WithMaxLevel(l.cfg.Rules.ControllerSecurityContextRule.GetLevel()))
	rules.NewRevisionHistoryLimitRule().ObjectRevisionHistoryLimit(object, errorList.WithRule("revision-history-limit").WithMaxLevel(l.cfg.Rules.NewRevisionHistoryLimitRule.GetLevel()))

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
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewNameDuplicatesRule().ContainerNameDuplicates(object, containers, errorList.WithMaxLevel(l.cfg.Rules.NameDuplicatesRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get()).
				ObjectReadOnlyRootFilesystem(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ReadOnlyRootFilesystemRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewNoNewPrivilegesRule(l.cfg.ExcludeRules.NoNewPrivileges.Get()).
				ContainerNoNewPrivileges(object, containers, errorList.WithMaxLevel(l.cfg.Rules.NoNewPrivilegesRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewSeccompProfileRule(l.cfg.ExcludeRules.SeccompProfile.Get()).
				ContainerSeccompProfile(object, containers, errorList.WithMaxLevel(l.cfg.Rules.SeccompProfileRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewHostNetworkPortsRule(l.cfg.ExcludeRules.HostNetworkPorts.Get()).ObjectHostNetworkPorts(object, containers, errorList.WithMaxLevel(l.cfg.Rules.HostNetworkPortsRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates(object, containers, errorList.WithMaxLevel(l.cfg.Rules.EnvVariablesDuplicatesRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewImageDigestRule(l.cfg.ExcludeRules.ImageDigest.Get()).ContainerImageDigestCheck(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ImageDigestRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewImagePullPolicyRule().ContainersImagePullPolicy(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ImagePullPolicyRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewResourcesRule(l.cfg.ExcludeRules.Resources.Get()).
				ContainerStorageEphemeral(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ResourcesRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewContainerSecurityContextRule(l.cfg.ExcludeRules.SecurityContext.Get()).
				ContainerSecurityContext(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ContainerSecurityContextRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewPortsRule(l.cfg.ExcludeRules.Ports.Get()).ContainerPorts(object, containers, errorList.WithMaxLevel(l.cfg.Rules.PortsRule.GetLevel()))
		},
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
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewLivenessRule(l.cfg.ExcludeRules.Liveness.Get()).
				CheckProbe(object, containers, errorList.WithMaxLevel(l.cfg.Rules.LivenessRule.GetLevel()))
		},
		func(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
			rules.NewReadinessRule(l.cfg.ExcludeRules.Readiness.Get()).
				CheckProbe(object, containers, errorList.WithMaxLevel(l.cfg.Rules.ReadinessRule.GetLevel()))
		},
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}
