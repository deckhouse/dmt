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

func (l *Container) applyContainerRules(object storage.StoreObject, moduleName string, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	// Register rules without exclusions in tracker if available
	if l.tracker != nil {
		l.tracker.RegisterExclusionsForModule(ID, "recommended-labels", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "namespace-labels", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "api-version", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "priority-class", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "revision-history-limit", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "name-duplicates", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "env-variables-duplicates", []string{}, moduleName)
		l.tracker.RegisterExclusionsForModule(ID, "image-pull-policy", []string{}, moduleName)
	}

	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		rules.NewRecommendedLabelsRule().ObjectRecommendedLabels,
		rules.NewNamespaceLabelsRule().ObjectNamespaceLabels,
		rules.NewAPIVersionRule().ObjectAPIVersion,
		rules.NewPriorityClassRule().ObjectPriorityClass,
		rules.NewDNSPolicyRuleWithTracker(l.cfg.ExcludeRules.DNSPolicy.Get(), l.tracker, ID, "dns-policy", moduleName).
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
