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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
)

func (l *Container) applyContainerRules(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		rules.NewRecommendedLabelsRule().ObjectRecommendedLabels,
		rules.NewNamespaceLabelsRule().ObjectNamespaceLabels,
		rules.NewAPIVersionRule().ObjectAPIVersion,
		rules.NewPriorityClassRule().ObjectPriorityClass,
		rules.NewDNSPolicyRule(l.cfg.ExcludeRules.DNSPolicy.Get()).
			ObjectDNSPolicy,
		rules.NewControllerSecurityContextRule(l.cfg.ExcludeRules.ControllerSecurityContext.Get()).
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
		rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get()).
			ObjectReadOnlyRootFilesystem,
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

func (l *Container) applyContainerRulesWithTracking(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	// Create tracked rules for exclusions
	dnsPolicyRule := exclusions.NewTrackedKindRule(
		l.cfg.ExcludeRules.DNSPolicy.Get(),
		l.tracker,
		ID,
		"dns-policy",
	)

	controllerSecurityContextRule := exclusions.NewTrackedKindRule(
		l.cfg.ExcludeRules.ControllerSecurityContext.Get(),
		l.tracker,
		ID,
		"controller-security-context",
	)

	readOnlyRootFilesystemRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get(),
		l.tracker,
		ID,
		"read-only-root-filesystem",
	)

	hostNetworkPortsRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.HostNetworkPorts.Get(),
		l.tracker,
		ID,
		"host-network-ports",
	)

	imageDigestRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.ImageDigest.Get(),
		l.tracker,
		ID,
		"image-digest",
	)

	resourcesRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.Resources.Get(),
		l.tracker,
		ID,
		"resources",
	)

	securityContextRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.SecurityContext.Get(),
		l.tracker,
		ID,
		"security-context",
	)

	portsRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.Ports.Get(),
		l.tracker,
		ID,
		"ports",
	)

	livenessRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.Liveness.Get(),
		l.tracker,
		ID,
		"liveness-probe",
	)

	readinessRule := exclusions.NewTrackedContainerRule(
		l.cfg.ExcludeRules.Readiness.Get(),
		l.tracker,
		ID,
		"readiness-probe",
	)

	// Register rules without exclusions in tracker
	l.tracker.RegisterExclusions(ID, "recommended-labels", []string{})
	l.tracker.RegisterExclusions(ID, "namespace-labels", []string{})
	l.tracker.RegisterExclusions(ID, "api-version", []string{})
	l.tracker.RegisterExclusions(ID, "priority-class", []string{})
	l.tracker.RegisterExclusions(ID, "revision-history-limit", []string{})
	l.tracker.RegisterExclusions(ID, "name-duplicates", []string{})
	l.tracker.RegisterExclusions(ID, "env-variables-duplicates", []string{})
	l.tracker.RegisterExclusions(ID, "image-pull-policy", []string{})

	// Apply object rules with tracking
	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		rules.NewRecommendedLabelsRule().ObjectRecommendedLabels,
		rules.NewNamespaceLabelsRule().ObjectNamespaceLabels,
		rules.NewAPIVersionRule().ObjectAPIVersion,
		rules.NewPriorityClassRule().ObjectPriorityClass,
		func(obj storage.StoreObject, errList *errors.LintRuleErrorsList) {
			if dnsPolicyRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
				rules.NewDNSPolicyRule([]pkg.KindRuleExclude{}).ObjectDNSPolicy(obj, errList)
			}
		},
		func(obj storage.StoreObject, errList *errors.LintRuleErrorsList) {
			if controllerSecurityContextRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
				rules.NewControllerSecurityContextRule([]pkg.KindRuleExclude{}).ControllerSecurityContext(obj, errList)
			}
		},
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

	// Apply container rules with tracking
	containerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		rules.NewNameDuplicatesRule().ContainerNameDuplicates,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if readOnlyRootFilesystemRule.Enabled(obj, &container) {
					rules.NewCheckReadOnlyRootFilesystemRule([]pkg.ContainerRuleExclude{}).ObjectReadOnlyRootFilesystem(obj, containers, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if hostNetworkPortsRule.Enabled(obj, &container) {
					rules.NewHostNetworkPortsRule([]pkg.ContainerRuleExclude{}).ObjectHostNetworkPorts(obj, containers, errList)
				}
			}
		},
		rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if imageDigestRule.Enabled(obj, &container) {
					rules.NewImageDigestRule([]pkg.ContainerRuleExclude{}).ContainerImageDigestCheck(obj, containers, errList)
				}
			}
		},
		rules.NewImagePullPolicyRule().ContainersImagePullPolicy,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if resourcesRule.Enabled(obj, &container) {
					rules.NewResourcesRule([]pkg.ContainerRuleExclude{}).ContainerStorageEphemeral(obj, containers, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if securityContextRule.Enabled(obj, &container) {
					rules.NewContainerSecurityContextRule([]pkg.ContainerRuleExclude{}).ContainerSecurityContext(obj, containers, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if portsRule.Enabled(obj, &container) {
					rules.NewPortsRule([]pkg.ContainerRuleExclude{}).ContainerPorts(obj, containers, errList)
				}
			}
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

	// Apply probe rules with tracking
	notInitContainerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if livenessRule.Enabled(obj, &container) {
					rules.NewLivenessRule([]pkg.ContainerRuleExclude{}).CheckProbe(obj, containers, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if readinessRule.Enabled(obj, &container) {
					rules.NewReadinessRule([]pkg.ContainerRuleExclude{}).CheckProbe(obj, containers, errList)
				}
			}
		},
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}
