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

func applyContainerRules(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := &errors.LintRuleErrorsList{}

	if slices.Contains(Cfg.SkipContainerChecks, object.Unstructured.GetName()) {
		return result
	}

	result.Add(objectRecommendedLabels(name, object))
	result.Add(namespaceLabels(name, object))
	result.Add(objectAPIVersion(name, object))
	result.Add(objectPriorityClass(name, object))
	result.Add(objectDNSPolicy(name, object))
	result.Add(objectSecurityContext(name, object))

	result.Add(objectRevisionHistoryLimit(name, object))
	result.Add(objectHostNetworkPorts(name, object))
	result.Add(objectServiceTargetPort(name, object))

	return result
}

func objectRecommendedLabels(name string, object storage.StoreObject) *errors.LintRuleError {
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			labels,
			`Object does not have the label "module"`,
		)
	}
	if _, ok := labels["heritage"]; !ok {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			labels,
			`Object does not have the label "heritage"`,
		)
	}
	return nil
}

func namespaceLabels(name string, object storage.StoreObject) *errors.LintRuleError {
	if object.Unstructured.GetKind() != "Namespace" {
		return nil
	}

	if !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return nil
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return nil
	}

	return errors.NewLintRuleError(
		ID,
		object.Identity(),
		name,
		labels,
		`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`)
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

func objectAPIVersion(name string, object storage.StoreObject) *errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	version := object.Unstructured.GetAPIVersion()

	switch kind {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return newAPIVersionError(name, "rbac.authorization.k8s.io/v1", version, object.Identity())
	case "Deployment", "DaemonSet", "StatefulSet":
		return newAPIVersionError(name, "apps/v1", version, object.Identity())
	case "Ingress":
		return newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity())
	case "PriorityClass":
		return newAPIVersionError(name, "scheduling.k8s.io/v1", version, object.Identity())
	case "PodSecurityPolicy":
		return newAPIVersionError(name, "policy/v1beta1", version, object.Identity())
	case "NetworkPolicy":
		return newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity())
	default:
		return nil
	}
}

func newConvertError(object storage.StoreObject, err error) *errors.LintRuleError {
	return errors.NewLintRuleError(
		ID,
		object.Identity(),
		object.Unstructured.GetName(),
		nil,
		"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
	)
}

func objectRevisionHistoryLimit(name string, object storage.StoreObject) *errors.LintRuleError {
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
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				nil,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		}

		if *actualLimit > maxHistoryLimit {
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				*actualLimit,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		}
	}
	return nil
}

func objectPriorityClass(name string, object storage.StoreObject) *errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	converter := runtime.DefaultUnstructuredConverter

	var priorityClass string

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = deployment.Spec.Template.Spec.PriorityClassName
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = daemonset.Spec.Template.Spec.PriorityClassName
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = statefulset.Spec.Template.Spec.PriorityClassName
	default:
		return nil
	}

	switch priorityClass {
	case "":
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			priorityClass,
			"Priority class must not be empty",
		)
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low" /* TODO: delete after migrating to 1.19 -> */, "cluster-critical":
	default:
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			priorityClass,
			"Priority class is not allowed",
		)
	}

	return nil
}

func objectSecurityContext(name string, object storage.StoreObject) *errors.LintRuleError {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return nil
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"GetPodSecurityContext failed: %v",
			err,
		)
	}

	if securityContext == nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext is not defined",
		)
	}
	if securityContext.RunAsNonRoot == nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsNonRoot",
		)
	}

	if securityContext.RunAsUser == nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsUser",
		)
	}
	if securityContext.RunAsGroup == nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"Object's SecurityContext missing parameter RunAsGroup",
		)
	}
	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)",
			)
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0",
			)
		}
	}

	return nil
}

func objectServiceTargetPort(name string, object storage.StoreObject) *errors.LintRuleError {
	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return nil
	}

	converter := runtime.DefaultUnstructuredConverter
	service := new(v1.Service)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service)
	if err != nil {
		panic(err)
	}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				return errors.NewLintRuleError(
					ID,
					object.Identity(),
					name,
					nil,
					"Service port must use an explicit named (non-numeric) target port",
				)
			}
			return errors.NewLintRuleError(
				ID,
				object.Identity(),
				name,
				port.TargetPort.IntVal,
				"Service port must use a named (non-numeric) target port",
			)
		}
	}
	return nil
}

func objectHostNetworkPorts(name string, object storage.StoreObject) *errors.LintRuleError {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return nil
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"IsHostNetwork failed: %v",
			err,
		)
	}

	containers, err := object.GetContainers()
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"GetContainers failed: %v",
			err,
		)
	}
	initContainers, err := object.GetInitContainers()
	if err != nil {
		return errors.NewLintRuleError(
			ID,
			object.Identity(),
			name,
			nil,
			"GetInitContainers failed: %v",
			err,
		)
	}
	containers = append(containers, initContainers...)

	for i := range containers {
		for _, p := range containers[i].Ports {
			if hostNetworkUsed && (p.ContainerPort < 4200 || p.ContainerPort >= 4300) {
				return errors.NewLintRuleError(
					ID,
					object.Identity()+" ; container = "+containers[i].Name,
					name,
					p.ContainerPort,
					"Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]",
				)
			}
			if p.HostPort != 0 && (p.HostPort < 4200 || p.HostPort >= 4300) {
				return errors.NewLintRuleError(
					ID,
					object.Identity()+" ; container = "+containers[i].Name,
					name,
					p.HostPort,
					"Container uses hostPort that doesn't fit the range [4200,4299]",
				)
			}
		}
	}

	return nil
}

func objectDNSPolicy(name string, object storage.StoreObject) *errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	converter := runtime.DefaultUnstructuredConverter

	var dnsPolicy string
	var hostNetwork bool

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(deployment.Spec.Template.Spec.DNSPolicy)
		hostNetwork = deployment.Spec.Template.Spec.HostNetwork
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(daemonset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = daemonset.Spec.Template.Spec.HostNetwork
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(statefulset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = statefulset.Spec.Template.Spec.HostNetwork
	default:
		return nil
	}

	if !hostNetwork {
		return nil
	}

	if dnsPolicy == "ClusterFirstWithHostNet" {
		return nil
	}

	return errors.NewLintRuleError(
		ID,
		object.Identity(),
		name,
		dnsPolicy,
		"dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`",
	)
}
