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

func (l *Container) applyContainerRulesWithTracking(object storage.StoreObject, moduleName string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	// Create tracked rules for exclusions
	dnsPolicyRule := exclusions.NewTrackedKindRuleForModule(
		l.cfg.ExcludeRules.DNSPolicy.Get(),
		l.tracker,
		ID,
		"dns-policy",
		moduleName,
	)

	controllerSecurityContextRule := exclusions.NewTrackedKindRuleForModule(
		l.cfg.ExcludeRules.ControllerSecurityContext.Get(),
		l.tracker,
		ID,
		"controller-security-context",
		moduleName,
	)

	readOnlyRootFilesystemRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get(),
		l.tracker,
		ID,
		"read-only-root-filesystem",
		moduleName,
	)

	hostNetworkPortsRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.HostNetworkPorts.Get(),
		l.tracker,
		ID,
		"host-network-ports",
		moduleName,
	)

	imageDigestRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.ImageDigest.Get(),
		l.tracker,
		ID,
		"image-digest",
		moduleName,
	)

	resourcesRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.Resources.Get(),
		l.tracker,
		ID,
		"resources",
		moduleName,
	)

	securityContextRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.SecurityContext.Get(),
		l.tracker,
		ID,
		"security-context",
		moduleName,
	)

	portsRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.Ports.Get(),
		l.tracker,
		ID,
		"ports",
		moduleName,
	)

	livenessRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.Liveness.Get(),
		l.tracker,
		ID,
		"liveness-probe",
		moduleName,
	)

	readinessRule := exclusions.NewTrackedContainerRuleForModule(
		l.cfg.ExcludeRules.Readiness.Get(),
		l.tracker,
		ID,
		"readiness-probe",
		moduleName,
	)

	// Register rules without exclusions in tracker
	l.tracker.RegisterExclusionsForModule(ID, "recommended-labels", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "namespace-labels", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "api-version", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "priority-class", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "revision-history-limit", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "name-duplicates", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "env-variables-duplicates", []string{}, moduleName)
	l.tracker.RegisterExclusionsForModule(ID, "image-pull-policy", []string{}, moduleName)

	// Apply object rules with tracking
	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		rules.NewRecommendedLabelsRule().ObjectRecommendedLabels,
		rules.NewNamespaceLabelsRule().ObjectNamespaceLabels,
		rules.NewAPIVersionRule().ObjectAPIVersion,
		rules.NewPriorityClassRule().ObjectPriorityClass,
		func(obj storage.StoreObject, errList *errors.LintRuleErrorsList) {
			// Create a rule with original exclusions for proper tracking
			dnsPolicyRuleWithExclusions := rules.NewDNSPolicyRule(l.cfg.ExcludeRules.DNSPolicy.Get())
			if dnsPolicyRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
				dnsPolicyRuleWithExclusions.ObjectDNSPolicy(obj, errList)
			}
		},
		func(obj storage.StoreObject, errList *errors.LintRuleErrorsList) {
			// Create a rule with original exclusions for proper tracking
			controllerSecurityContextRuleWithExclusions := rules.NewControllerSecurityContextRule(l.cfg.ExcludeRules.ControllerSecurityContext.Get())
			if controllerSecurityContextRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
				controllerSecurityContextRuleWithExclusions.ControllerSecurityContext(obj, errList)
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
			readOnlyRootFilesystemRuleWithExclusions := rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get())
			for i := range containers {
				container := &containers[i]
				if readOnlyRootFilesystemRule.Enabled(obj, container) {
					readOnlyRootFilesystemRuleWithExclusions.ObjectReadOnlyRootFilesystem(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			hostNetworkPortsRuleWithExclusions := rules.NewHostNetworkPortsRule(l.cfg.ExcludeRules.HostNetworkPorts.Get())
			for i := range containers {
				container := &containers[i]
				if hostNetworkPortsRule.Enabled(obj, container) {
					hostNetworkPortsRuleWithExclusions.ObjectHostNetworkPorts(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			imageDigestRuleWithExclusions := rules.NewImageDigestRule(l.cfg.ExcludeRules.ImageDigest.Get())
			for i := range containers {
				container := &containers[i]
				if imageDigestRule.Enabled(obj, container) {
					imageDigestRuleWithExclusions.ContainerImageDigestCheck(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		rules.NewImagePullPolicyRule().ContainersImagePullPolicy,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			resourcesRuleWithExclusions := rules.NewResourcesRule(l.cfg.ExcludeRules.Resources.Get())
			for i := range containers {
				container := &containers[i]
				if resourcesRule.Enabled(obj, container) {
					resourcesRuleWithExclusions.ContainerStorageEphemeral(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			securityContextRuleWithExclusions := rules.NewContainerSecurityContextRule(l.cfg.ExcludeRules.SecurityContext.Get())
			for i := range containers {
				container := &containers[i]
				if securityContextRule.Enabled(obj, container) {
					securityContextRuleWithExclusions.ContainerSecurityContext(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			portsRuleWithExclusions := rules.NewPortsRule(l.cfg.ExcludeRules.Ports.Get())
			for i := range containers {
				container := &containers[i]
				if portsRule.Enabled(obj, container) {
					portsRuleWithExclusions.ContainerPorts(obj, []corev1.Container{*container}, errList)
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
			livenessRuleWithExclusions := rules.NewLivenessRule(l.cfg.ExcludeRules.Liveness.Get())
			for i := range containers {
				container := &containers[i]
				if livenessRule.Enabled(obj, container) {
					livenessRuleWithExclusions.CheckProbe(obj, []corev1.Container{*container}, errList)
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			readinessRuleWithExclusions := rules.NewReadinessRule(l.cfg.ExcludeRules.Readiness.Get())
			for i := range containers {
				container := &containers[i]
				if readinessRule.Enabled(obj, container) {
					readinessRuleWithExclusions.CheckProbe(obj, []corev1.Container{*container}, errList)
				}
			}
		},
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}
