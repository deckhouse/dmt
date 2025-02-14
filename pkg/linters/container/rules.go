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
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
)

const defaultRegistry = "registry.example.com/deckhouse"

const (
	objectRevisionHistoryLimitRuleName      = "object-revision-history-limit"
	objectSecurityContextRuleName           = "object-security-context"
	checkSecurityContextParametersRuleName  = "check-security-context-parameters"
	checkRunAsNonRootRuleName               = "check-run-as-non-root"
	containerPortsRuleName                  = "ports"
	containerImagePullPolicyRuleName        = "image-pull-policy"
	containerImageDigestCheckRuleName       = "image-digest-check"
	containerEnvVariablesDuplicatesRuleName = "env-variables-duplicates"
	objectHostNetworkPortsRuleName          = "host-network-ports"
	containerNameDuplicatesRuleName         = "name-duplicates"
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
		containerNameDuplicates,
		rules.NewCheckReadOnlyRootFilesystemRule(l.cfg.ExcludeRules.ReadOnlyRootFilesystem.Get()).
			ObjectReadOnlyRootFilesystem,
		objectHostNetworkPorts,

		// old with module names skipping
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		rules.NewResourcesRule(l.cfg.ExcludeRules.Resources.Get()).
			ContainerStorageEphemeral,
		rules.NewContainerSecurityContextRule(l.cfg.ExcludeRules.SecurityContext.Get()).
			ContainerSecurityContext,
		containerPorts,
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

func containersImagePullPolicy(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(containerImagePullPolicyRuleName)

	if object.Unstructured.GetNamespace() == "d8-system" && object.Unstructured.GetKind() == "Deployment" && object.Unstructured.GetName() == "deckhouse" {
		checkImagePullPolicyAlways(object, containers, errorList)

		return
	}

	containerImagePullPolicyIfNotPresent(object, containers, errorList)
}

func checkImagePullPolicyAlways(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	c := containers[0]
	if c.ImagePullPolicy != corev1.PullAlways {
		errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(c.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "Always"`)
	}
}

func containerNameDuplicates(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(containerNameDuplicatesRuleName)

	if hasDuplicates(containers, func(c corev1.Container) string { return c.Name }) {
		errorList.WithObjectID(object.Identity()).
			Error("Duplicate container name")
	}
}

func containerEnvVariablesDuplicates(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(containerEnvVariablesDuplicatesRuleName)

	for i := range containers {
		c := &containers[i]

		if hasDuplicates(c.Env, func(e corev1.EnvVar) string { return e.Name }) {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Container has two env variables with same name")

			return
		}
	}
}

func hasDuplicates[T any](items []T, keyFunc func(T) string) bool {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
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

func containerImageDigestCheck(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(containerImageDigestCheckRuleName)

	for i := range containers {
		c := &containers[i]

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + c.Name).
				Error("Cannot parse repository from image")

			return
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			errorList.WithObjectID(object.Identity()+"; container = "+c.Name).
				Errorf("Cannot parse repository from image: %s", c.Image)

			return
		}

		if repo.Name() != defaultRegistry {
			errorList.WithObjectID(object.Identity()+"; container = "+c.Name).
				Errorf("All images must be deployed from the same default registry: %s current: %s", defaultRegistry, repo.RepositoryStr())

			return
		}
	}
}

func containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]

		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(c.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
}

func containerPorts(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(containerPortsRuleName)

	const t = 1024
	for i := range containers {
		c := &containers[i]

		for _, port := range c.Ports {
			if port.ContainerPort <= t {
				errorList.WithObjectID(object.Identity() + "; container = " + c.Name).WithValue(port.ContainerPort).
					Error("Container uses port <= 1024")

				return
			}
		}
	}
}

func objectHostNetworkPorts(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(objectHostNetworkPortsRuleName)

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("IsHostNetwork failed: %v", err)

		return
	}

	for i := range containers {
		c := &containers[i]

		for _, port := range c.Ports {
			if hostNetworkUsed && (port.ContainerPort < 4200 || port.ContainerPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithValue(port.ContainerPort).
					Error("Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]")
			}
			if port.HostPort != 0 && (port.HostPort < 4200 || port.HostPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + c.Name).WithValue(port.HostPort).
					Error("Container uses hostPort that doesn't fit the range [4200,4299]")
			}
		}
	}
}
