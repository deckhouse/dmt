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

package rules

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SeccompProfileRuleName = "seccomp-profile"
)

func NewSeccompProfileRule(excludeRules []pkg.ContainerRuleExclude) *SeccompProfileRule {
	return &SeccompProfileRule{
		RuleMeta: pkg.RuleMeta{
			Name: SeccompProfileRuleName,
		},
		ContainerRule: pkg.ContainerRule{
			ExcludeRules: excludeRules,
		},
	}
}

type SeccompProfileRule struct {
	pkg.RuleMeta
	pkg.ContainerRule
}

// ContainerSeccompProfile checks that containers use the default seccomp profile and don't disable seccomp
// This ensures containers are protected by the default seccomp profile which filters system calls
// Reference: CIS Kubernetes Benchmark 5.7.2, OWASP Kubernetes Security Cheat Sheet
func (r *SeccompProfileRule) ContainerSeccompProfile(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName()).WithFilePath(object.ShortPath())

	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	// Check pod-level seccomp profile first
	podSeccompProfile, err := r.getPodSeccompProfile(object)
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Failed to get pod seccomp profile: %v", err)
		return
	}

	for i := range containers {
		c := &containers[i]

		if !r.Enabled(object, c) {
			// TODO: add metrics
			continue
		}

		// Check container-level seccomp profile
		containerSeccompProfile := r.getContainerSeccompProfile(c)

		// Determine effective seccomp profile (container-level overrides pod-level)
		effectiveProfile := containerSeccompProfile
		if effectiveProfile == nil {
			effectiveProfile = podSeccompProfile
		}

		r.validateSeccompProfile(effectiveProfile, object, c, errorList)
	}
}

// getPodSeccompProfile extracts seccomp profile from pod security context
func (*SeccompProfileRule) getPodSeccompProfile(object storage.StoreObject) (*corev1.SeccompProfile, error) {
	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		return nil, err
	}

	if securityContext == nil {
		return nil, nil
	}

	return securityContext.SeccompProfile, nil
}

// getContainerSeccompProfile extracts seccomp profile from container security context
func (*SeccompProfileRule) getContainerSeccompProfile(container *corev1.Container) *corev1.SeccompProfile {
	if container.SecurityContext == nil {
		return nil
	}

	return container.SecurityContext.SeccompProfile
}

// validateSeccompProfile validates the effective seccomp profile
func (*SeccompProfileRule) validateSeccompProfile(profile *corev1.SeccompProfile, object storage.StoreObject, container *corev1.Container, errorList *errors.LintRuleErrorsList) {
	objectID := object.Identity() + " ; container = " + container.Name

	if profile == nil {
		// No seccomp profile specified - this is acceptable as Kubernetes will use the default
		// RuntimeDefault profile in newer versions, but we'll warn for explicit configuration
		errorList.WithObjectID(objectID).
			Warn("No seccomp profile specified - consider explicitly setting seccompProfile.type to 'RuntimeDefault' for better security posture")
		return
	}

	switch profile.Type {
	case corev1.SeccompProfileTypeRuntimeDefault:
		// This is the recommended secure default - no error
		return

	case corev1.SeccompProfileTypeUnconfined:
		// Unconfined disables seccomp filtering, which is a security risk
		errorList.WithObjectID(objectID).
			Error("Container has seccompProfile.type set to 'Unconfined' which disables seccomp filtering and poses security risks - use 'RuntimeDefault' instead")

	case corev1.SeccompProfileTypeLocalhost:
		// Custom profile - warn about proper validation
		if profile.LocalhostProfile == nil || *profile.LocalhostProfile == "" {
			errorList.WithObjectID(objectID).
				Error("Container has seccompProfile.type set to 'Localhost' but localhostProfile is empty")
		} else {
			errorList.WithObjectID(objectID).
				Warn("Container uses custom seccomp profile - ensure it's properly configured and maintained")
		}

	default:
		errorList.WithObjectID(objectID).
			Errorf("Container has unknown seccompProfile.type: %s", profile.Type)
	}
}
