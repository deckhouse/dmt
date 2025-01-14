package k8sresources

import (
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

func applyContainerRules(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	if slices.Contains(Cfg.SkipContainerChecks, object.Unstructured.GetName()) {
		return errors.LintRuleErrorsList{}
	}

	rules := []func(string, storage.StoreObject) errors.LintRuleErrorsList{
		objectRecommendedLabels,
		namespaceLabels,
		objectAPIVersion,
		objectPriorityClass,
		objectDNSPolicy,
		objectSecurityContext,
		objectReadOnlyRootFilesystem,
		objectRevisionHistoryLimit,
		objectHostNetworkPorts,
		objectServiceTargetPort,
	}

	for _, rule := range rules {
		result.Merge(rule(name, object))
	}

	return result
}

func objectRecommendedLabels(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			labels,
			`Object does not have the label "module"`,
		))
	}
	if _, ok := labels["heritage"]; !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			labels,
			`Object does not have the label "heritage"`,
		))
	}

	return result
}

func namespaceLabels(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	if object.Unstructured.GetKind() != "Namespace" {
		return result
	}

	if !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return result
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return result
	}

	result.Add(errors.NewLintRuleError(
		ID,
		object.Identity(),
		name,
		labels,
		`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`))

	return result
}

func newAPIVersionError(name, wanted, version, objectID string) *errors.LintRuleError {
	if version != wanted {
		return errors.NewLintRuleError(
			ID,
			objectID,
			name,
			nil,
			"Object defined using deprecated api version, wanted %q", wanted,
		)
	}
	return nil
}

func objectAPIVersion(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	kind := object.Unstructured.GetKind()
	version := object.Unstructured.GetAPIVersion()

	switch kind {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		result.Add(newAPIVersionError(name, "rbac.authorization.k8s.io/v1", version, object.Identity()))
	case "Deployment", "DaemonSet", "StatefulSet":
		result.Add(newAPIVersionError(name, "apps/v1", version, object.Identity()))
	case "Ingress":
		result.Add(newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity()))
	case "PriorityClass":
		result.Add(newAPIVersionError(name, "scheduling.k8s.io/v1", version, object.Identity()))
	case "PodSecurityPolicy":
		result.Add(newAPIVersionError(name, "policy/v1beta1", version, object.Identity()))
	case "NetworkPolicy":
		result.Add(newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity()))
	}

	return result
}

func newConvertError(object storage.StoreObject, err error) (result errors.LintRuleErrorsList) {
	result.Add(errors.NewLintRuleError(
		ID,
		object.Identity(),
		object.Unstructured.GetName(),
		nil,
		"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
	))

	return result
}

func objectRevisionHistoryLimit(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit
		// Revision history limit controls the number of replicasets stored in the cluster for each deployment.
		// Higher number means higher resource consumption, lower means inability to rollback.
		//
		// Since Deckhouse does not use rollback, we can set it to 2 to be able to manually check the previous version.
		// It is more important to reduce the control plane pressure.
		maxHistoryLimit := int32(2)
		actualLimit := deployment.Spec.RevisionHistoryLimit

		if actualLimit == nil {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				nil,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			))
		} else if *actualLimit > maxHistoryLimit {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				*actualLimit,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			))
		}
	}

	return result
}

func objectPriorityClass(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	if !isPriorityClassSupportedKind(object.Unstructured.GetKind()) {
		return result
	}

	priorityClass, err := getPriorityClass(object)
	if err != nil {
		return newConvertError(object, err)
	}

	return validatePriorityClass(priorityClass, name, object, result)
}

func isPriorityClassSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet":
		return true
	default:
		return false
	}
}

func validatePriorityClass(priorityClass, name string, object storage.StoreObject, result errors.LintRuleErrorsList) errors.LintRuleErrorsList {
	switch priorityClass {
	case "":
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			priorityClass,
			"Priority class must not be empty",
		))
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "cluster-critical":
	default:
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			priorityClass,
			"Priority class is not allowed",
		))
	}
	return result
}

func getPriorityClass(object storage.StoreObject) (string, error) {
	kind := object.Unstructured.GetKind()
	converter := runtime.DefaultUnstructuredConverter

	var priorityClass string
	var err error

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		priorityClass = deployment.Spec.Template.Spec.PriorityClassName
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		priorityClass = daemonset.Spec.Template.Spec.PriorityClassName
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		priorityClass = statefulset.Spec.Template.Spec.PriorityClassName
	}

	return priorityClass, err
}

func objectSecurityContext(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	if !isSupportedKind(object.Unstructured.GetKind()) {
		return result
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		return addSecurityContextError(result, object, name, "GetPodSecurityContext failed: %v", err)
	}

	if securityContext == nil {
		return addSecurityContextError(result, object, name, "Object's SecurityContext is not defined")
	}

	checkSecurityContextParameters(securityContext, result, object, name)

	return result
}

func isSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func addSecurityContextError(result errors.LintRuleErrorsList, object storage.StoreObject, name, message string, args ...any) errors.LintRuleErrorsList {
	result.Add(errors.NewLintRuleError(
		ID,
		object.Identity(),
		name,
		nil,
		message,
		args...,
	))
	return result
}

func checkSecurityContextParameters(securityContext *v1.PodSecurityContext, result errors.LintRuleErrorsList, object storage.StoreObject, name string) {
	if securityContext.RunAsNonRoot == nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsNonRoot",
		))
	}

	if securityContext.RunAsUser == nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsUser",
		))
	}
	if securityContext.RunAsGroup == nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsGroup",
		))
	}

	if securityContext.RunAsNonRoot != nil && securityContext.RunAsUser != nil && securityContext.RunAsGroup != nil {
		checkRunAsNonRoot(securityContext, result, object, name)
	}
}

func checkRunAsNonRoot(securityContext *v1.PodSecurityContext, result errors.LintRuleErrorsList, object storage.StoreObject, name string) {
	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)",
			))
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0",
			))
		}
	}
}

func objectReadOnlyRootFilesystem(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return result
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"GetAllContainers failed: %v",
			err,
		))

		return result
	}

	for i := range containers {
		c := &containers[i]
		if c.VolumeMounts == nil {
			continue
		}
		if c.SecurityContext == nil {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity()+" ; container = "+containers[i].Name,
				name,
				nil,
				"Container's SecurityContext is missing",
			))
			continue
		}
		if c.SecurityContext.ReadOnlyRootFilesystem == nil {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity()+" ; container = "+containers[i].Name,
				name,
				nil,
				"Container's SecurityContext missing parameter ReadOnlyRootFilesystem",
			))
			continue
		}
		if !*c.SecurityContext.ReadOnlyRootFilesystem {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity()+" ; container = "+containers[i].Name,
				name,
				nil,
				"Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`",
			))
		}
	}

	return result
}

func objectServiceTargetPort(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return result
	}

	converter := runtime.DefaultUnstructuredConverter
	service := new(v1.Service)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service)
	if err != nil {
		return newConvertError(object, err)
	}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				result.Add(errors.NewLintRuleError(
					ID,
					object.Identity(),
					name,
					nil,
					"Service port must use an explicit named (non-numeric) target port",
				))

				continue
			}
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				port.TargetPort.IntVal,
				"Service port must use a named (non-numeric) target port",
			))
		}
	}

	return result
}

func objectHostNetworkPorts(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return result
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"IsHostNetwork failed: %v",
			err,
		))

		return result
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"GetAllContainers failed: %v",
			err,
		))

		return result
	}

	for i := range containers {
		for _, p := range containers[i].Ports {
			if hostNetworkUsed && (p.ContainerPort < 4200 || p.ContainerPort >= 4300) {
				result.Add(errors.NewLintRuleError(
					ID,
					object.Identity()+" ; container = "+containers[i].Name,
					name,
					p.ContainerPort,
					"Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]",
				))
			}
			if p.HostPort != 0 && (p.HostPort < 4200 || p.HostPort >= 4300) {
				result.Add(errors.NewLintRuleError(
					ID,
					object.Identity()+" ; container = "+containers[i].Name,
					name,
					p.HostPort,
					"Container uses hostPort that doesn't fit the range [4200,4299]",
				))
			}
		}
	}

	return result
}

func objectDNSPolicy(name string, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	dnsPolicy, hostNetwork, err := getDNSPolicyAndHostNetwork(object)
	if err != nil {
		return newConvertError(object, err)
	}

	return validateDNSPolicy(dnsPolicy, hostNetwork, name, object, result)
}

func validateDNSPolicy(dnsPolicy string, hostNetwork bool, name string, object storage.StoreObject, result errors.LintRuleErrorsList) errors.LintRuleErrorsList {
	if !hostNetwork {
		return result
	}

	if dnsPolicy != "ClusterFirstWithHostNet" {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			dnsPolicy,
			"dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`",
		))
	}

	return result
}

func getDNSPolicyAndHostNetwork(object storage.StoreObject) (dnsPolicy string, hostNetwork bool, err error) {
	kind := object.Unstructured.GetKind()
	converter := runtime.DefaultUnstructuredConverter

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		dnsPolicy = string(deployment.Spec.Template.Spec.DNSPolicy)
		hostNetwork = deployment.Spec.Template.Spec.HostNetwork
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		dnsPolicy = string(daemonset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = daemonset.Spec.Template.Spec.HostNetwork
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err = converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		dnsPolicy = string(statefulset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = statefulset.Spec.Template.Spec.HostNetwork
	}

	return dnsPolicy, hostNetwork, err
}
