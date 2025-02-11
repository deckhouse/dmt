package k8sresources

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

func applyContainerRules(object storage.StoreObject, lintError *errors.Error) {
	rules := []func(storage.StoreObject, *errors.Error){
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
		rule(object, lintError)
	}
}

func objectRecommendedLabels(object storage.StoreObject, lintError *errors.Error) {
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		lintError.WithObjectID(object.Identity()).WithValue(labels).
			Add(`Object does not have the label "module"`)
	}
	if _, ok := labels["heritage"]; !ok {
		lintError.WithObjectID(object.Identity()).WithValue(labels).
			Add(`Object does not have the label "heritage"`)
	}
}

func namespaceLabels(object storage.StoreObject, lintError *errors.Error) {
	if object.Unstructured.GetKind() != "Namespace" {
		return
	}

	if !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return
	}

	lintError.WithObjectID(object.Identity()).WithValue(labels).
		Add(`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`)
}

func newAPIVersionError(wanted, version, objectID string, lintError *errors.Error) {
	if version != wanted {
		lintError.WithObjectID(objectID).Add(
			"Object defined using deprecated api version, wanted %q", wanted,
		)
	}
}

func objectAPIVersion(object storage.StoreObject, lintError *errors.Error) {
	kind := object.Unstructured.GetKind()
	version := object.Unstructured.GetAPIVersion()

	switch kind {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		newAPIVersionError("rbac.authorization.k8s.io/v1", version, object.Identity(), lintError)
	case "Deployment", "DaemonSet", "StatefulSet":
		newAPIVersionError("apps/v1", version, object.Identity(), lintError)
	case "Ingress":
		newAPIVersionError("networking.k8s.io/v1", version, object.Identity(), lintError)
	case "PriorityClass":
		newAPIVersionError("scheduling.k8s.io/v1", version, object.Identity(), lintError)
	case "PodSecurityPolicy":
		newAPIVersionError("policy/v1beta1", version, object.Identity(), lintError)
	case "NetworkPolicy":
		newAPIVersionError("networking.k8s.io/v1", version, object.Identity(), lintError)
	}
}

func objectRevisionHistoryLimit(object storage.StoreObject, lintError *errors.Error) {
	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			lintError.WithObjectID(object.Unstructured.GetName()).Add(
				"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
			)
			return
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
			lintError.WithObjectID(object.Identity()).Add(
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		} else if *actualLimit > maxHistoryLimit {
			lintError.WithObjectID(object.Identity()).WithValue(*actualLimit).
				Add("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		}
	}
}

func objectPriorityClass(object storage.StoreObject, lintError *errors.Error) {
	if !isPriorityClassSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	priorityClass, err := getPriorityClass(object)
	if err != nil {
		lintError.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
		return
	}

	validatePriorityClass(priorityClass, object, lintError)
}

func isPriorityClassSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet":
		return true
	default:
		return false
	}
}

func validatePriorityClass(priorityClass string, object storage.StoreObject, lintError *errors.Error) {
	switch priorityClass {
	case "":
		lintError.WithObjectID(object.Identity()).WithValue(priorityClass).
			Add("Priority class must not be empty")
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "cluster-critical":
	default:
		lintError.WithObjectID(object.Identity()).WithValue(priorityClass).
			Add("Priority class is not allowed")
	}
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

func objectSecurityContext(object storage.StoreObject, lintError *errors.Error) {
	if !isSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		lintError.WithObjectID(object.Identity()).Add("GetPodSecurityContext failed: %v", err)
		return
	}

	if securityContext == nil {
		lintError.WithObjectID(object.Identity()).Add("Object's SecurityContext is not defined")
		return
	}

	checkSecurityContextParameters(securityContext, object, lintError)
}

func isSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func checkSecurityContextParameters(securityContext *v1.PodSecurityContext, object storage.StoreObject, lintError *errors.Error) {
	if securityContext.RunAsNonRoot == nil {
		lintError.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsNonRoot")
	}

	if securityContext.RunAsUser == nil {
		lintError.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsUser")
	}
	if securityContext.RunAsGroup == nil {
		lintError.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsGroup")
	}

	if securityContext.RunAsNonRoot != nil && securityContext.RunAsUser != nil && securityContext.RunAsGroup != nil {
		checkRunAsNonRoot(securityContext, object, lintError)
	}
}

func checkRunAsNonRoot(securityContext *v1.PodSecurityContext, object storage.StoreObject, lintError *errors.Error) {
	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			lintError.WithObjectID(object.Identity()).
				WithValue(fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)).
				Add("Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)")
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			lintError.WithObjectID(object.Identity()).
				WithValue(fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)).
				Add("Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0")
		}
	}
}

func objectReadOnlyRootFilesystem(object storage.StoreObject, lintError *errors.Error) {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		lintError.WithObjectID(object.Identity()).Add("GetAllContainers failed: %v", err)
		return
	}

	for i := range containers {
		c := &containers[i]
		if c.VolumeMounts == nil {
			continue
		}
		if c.SecurityContext == nil {
			lintError.WithObjectID(object.Identity()).Add("Container's SecurityContext is missing")
			continue
		}
		if c.SecurityContext.ReadOnlyRootFilesystem == nil {
			lintError.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
				Add("Container's SecurityContext missing parameter ReadOnlyRootFilesystem")
			continue
		}
		if !*c.SecurityContext.ReadOnlyRootFilesystem {
			lintError.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).Add(
				"Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`",
			)
		}
	}
}

func objectServiceTargetPort(object storage.StoreObject, lintError *errors.Error) {
	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return
	}

	converter := runtime.DefaultUnstructuredConverter
	service := new(v1.Service)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service)
	if err != nil {
		lintError.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
		return
	}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				lintError.WithObjectID(object.Identity()).Add(
					"Service port must use an explicit named (non-numeric) target port",
				)

				continue
			}
			lintError.WithObjectID(object.Identity()).WithValue(port.TargetPort.IntVal).
				Add("Service port must use a named (non-numeric) target port")
		}
	}
}

func objectHostNetworkPorts(object storage.StoreObject, lintError *errors.Error) {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		lintError.WithObjectID(object.Identity()).Add("IsHostNetwork failed: %v", err)
		return
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		lintError.WithObjectID(object.Identity()).Add("GetAllContainers failed: %v", err)
		return
	}

	for i := range containers {
		for _, p := range containers[i].Ports {
			if hostNetworkUsed && (p.ContainerPort < 4200 || p.ContainerPort >= 4300) {
				lintError.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
					WithValue(p.ContainerPort).
					Add("Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]")
			}
			if p.HostPort != 0 && (p.HostPort < 4200 || p.HostPort >= 4300) {
				lintError.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
					WithValue(p.HostPort).
					Add("Container uses hostPort that doesn't fit the range [4200,4299]")
			}
		}
	}
}

func objectDNSPolicy(object storage.StoreObject, lintError *errors.Error) {
	dnsPolicy, hostNetwork, err := getDNSPolicyAndHostNetwork(object)
	if err != nil {
		lintError.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
		return
	}

	validateDNSPolicy(dnsPolicy, hostNetwork, object, lintError)
}

func validateDNSPolicy(dnsPolicy string, hostNetwork bool, object storage.StoreObject, lintError *errors.Error) {
	if !hostNetwork {
		return
	}

	if dnsPolicy != "ClusterFirstWithHostNet" {
		lintError.WithObjectID(object.Identity()).WithValue(dnsPolicy).
			Add("dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`")
	}
}

func getDNSPolicyAndHostNetwork(object storage.StoreObject) (string, bool, error) { //nolint:gocritic // false positive
	var dnsPolicy string
	var hostNetwork bool
	var err error
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
