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
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
	corev1 "k8s.io/api/core/v1"
)

// ContainerTracked linter with exclusion tracking
type ContainerTracked struct {
	name, desc string
	cfg        *config.ContainerSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

// NewTracked creates a new container linter with exclusion tracking
func NewTracked(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList, tracker *exclusions.ExclusionTracker) *ContainerTracked {
	return &ContainerTracked{
		name:      ID,
		desc:      "Lint container objects with exclusion tracking",
		cfg:       &cfg.LintersSettings.Container,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Container.Impact),
		tracker:   tracker,
	}
}

func (l *ContainerTracked) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())
	for _, object := range m.GetStorage() {
		l.applyContainerRulesTracked(object, errorList)
	}
}

func (l *ContainerTracked) Name() string {
	return l.name
}

func (l *ContainerTracked) Desc() string {
	return l.desc
}

func (l *ContainerTracked) applyContainerRulesTracked(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithFilePath(object.ShortPath())

	// Use tracked rules for exclusions
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

	// Apply rules using tracked exclusions
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

	containerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		rules.NewNameDuplicatesRule().ContainerNameDuplicates,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if readOnlyRootFilesystemRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if hostNetworkPortsRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		rules.NewEnvVariablesDuplicatesRule().ContainerEnvVariablesDuplicates,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if imageDigestRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		rules.NewImagePullPolicyRule().ContainersImagePullPolicy,
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if resourcesRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if securityContextRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if portsRule.Enabled(obj, &container) {
					// Apply the rule logic here
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

	notInitContainerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if livenessRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
		func(obj storage.StoreObject, containers []corev1.Container, errList *errors.LintRuleErrorsList) {
			for _, container := range containers {
				if readinessRule.Enabled(obj, &container) {
					// Apply the rule logic here
				}
			}
		},
	}

	for _, rule := range notInitContainerRules {
		rule(object, containers, errorList)
	}
}
