package container

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func applyContainerRules(moduleName string, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	objectRules := []func(storage.StoreObject, *errors.LintRuleErrorsList){
		objectRecommendedLabels,
		namespaceLabels,
		objectAPIVersion,
		objectPriorityClass,
		objectDNSPolicy,
		objectSecurityContext,
		objectRevisionHistoryLimit,
		objectServiceTargetPort,
	}

	for _, rule := range objectRules {
		rule(object, errorList)
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Cannot get containers from object: %s", err)

		return
	}

	if len(containers) == 0 {
		return
	}

	containerRules := []func(storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		containerNameDuplicates,
		objectReadOnlyRootFilesystem,
		objectHostNetworkPorts,
	}

	for _, rule := range containerRules {
		rule(object, containers, errorList)
	}

	containerRulesWithModuleName := []func(string, storage.StoreObject, []corev1.Container, *errors.LintRuleErrorsList){
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		containerStorageEphemeral,
		containerSecurityContext,
		containerPorts,
	}

	for _, rule := range containerRulesWithModuleName {
		rule(moduleName, object, containers, errorList)
	}
}

func containersImagePullPolicy(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	if object.Unstructured.GetNamespace() == "d8-system" && object.Unstructured.GetKind() == "Deployment" && object.Unstructured.GetName() == "deckhouse" {
		checkImagePullPolicyAlways(object, containers[0], errorList)

		return
	}

	containerImagePullPolicyIfNotPresent(moduleName, object, containers, errorList)
}

func checkImagePullPolicyAlways(object storage.StoreObject, container corev1.Container, errorList *errors.LintRuleErrorsList) {
	if container.ImagePullPolicy != corev1.PullAlways {
		errorList.WithObjectID(object.Identity() + "; container = " + container.Name).WithValue(container.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "Always"`)
	}
}

func containerNameDuplicates(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	if hasDuplicates(containers, func(c corev1.Container) string { return c.Name }) {
		errorList.WithObjectID(object.Identity()).
			Error("Duplicate container name")
	}
}

func containerEnvVariablesDuplicates(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}
		if hasDuplicates(container.Env, func(e corev1.EnvVar) string { return e.Name }) {
			errorList.WithObjectID(object.Identity() + "; container = " + container.Name).
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

func shouldSkipModuleContainer(moduleName, container string) bool {
	for _, line := range Cfg.SkipContainers {
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

func containerImageDigestCheck(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(container.Image)
		if len(match) == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + container.Name).
				Error("Cannot parse repository from image")

			return
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			errorList.WithObjectID(object.Identity()+"; container = "+container.Name).
				Errorf("Cannot parse repository from image: %s", container.Image)

			return
		}

		if repo.Name() != defaultRegistry {
			errorList.WithObjectID(object.Identity()+"; container = "+container.Name).
				Errorf("All images must be deployed from the same default registry: %s current: %s", defaultRegistry, repo.RepositoryStr())

			return
		}
	}
}

func containerImagePullPolicyIfNotPresent(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}
		if container.ImagePullPolicy == "" || container.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		errorList.WithObjectID(object.Identity() + "; container = " + container.Name).WithValue(container.ImagePullPolicy).
			Error(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
}

func containerStorageEphemeral(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}
		if container.Resources.Requests.StorageEphemeral() == nil || container.Resources.Requests.StorageEphemeral().Value() == 0 {
			errorList.WithObjectID(object.Identity() + "; container = " + container.Name).
				Error("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
}

func containerSecurityContext(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}
		if container.SecurityContext == nil {
			errorList.WithObjectID(object.Identity() + "; container = " + container.Name).
				Error("Container SecurityContext is not defined")

			return
		}
	}
}

func containerPorts(moduleName string, object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	const t = 1024
	for _, container := range containers {
		if shouldSkipModuleContainer(moduleName, container.Name) {
			continue
		}
		for _, port := range container.Ports {
			if port.ContainerPort <= t {
				errorList.WithObjectID(object.Identity() + "; container = " + container.Name).WithValue(port.ContainerPort).
					Error("Container uses port <= 1024")

				return
			}
		}
	}
}

func objectReadOnlyRootFilesystem(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return
	}

	for _, container := range containers {
		if container.VolumeMounts == nil {
			continue
		}
		if container.SecurityContext == nil {
			errorList.WithObjectID(object.Identity()).
				Error("Container's SecurityContext is missing")

			continue
		}
		if container.SecurityContext.ReadOnlyRootFilesystem == nil {
			errorList.WithObjectID(object.Identity() + " ; container = " + container.Name).
				Error("Container's SecurityContext missing parameter ReadOnlyRootFilesystem")

			continue
		}
		if !*container.SecurityContext.ReadOnlyRootFilesystem {
			errorList.WithObjectID(object.Identity() + " ; container = " + container.Name).
				Error("Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`")
		}
	}
}

func objectHostNetworkPorts(object storage.StoreObject, containers []corev1.Container, errorList *errors.LintRuleErrorsList) {
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

	for _, container := range containers {
		for _, port := range container.Ports {
			if hostNetworkUsed && (port.ContainerPort < 4200 || port.ContainerPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + container.Name).WithValue(port.ContainerPort).
					Error("Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]")
			}
			if port.HostPort != 0 && (port.HostPort < 4200 || port.HostPort >= 4300) {
				errorList.WithObjectID(object.Identity() + " ; container = " + container.Name).WithValue(port.HostPort).
					Error("Container uses hostPort that doesn't fit the range [4200,4299]")
			}
		}
	}
}

func objectRecommendedLabels(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		errorList.WithObjectID(object.Identity()).WithValue(labels).
			Error(`Object does not have the label "module"`)
	}
	if _, ok := labels["heritage"]; !ok {
		errorList.WithObjectID(object.Identity()).WithValue(labels).
			Error(`Object does not have the label "heritage"`)
	}
}

func namespaceLabels(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if object.Unstructured.GetKind() != "Namespace" || !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return
	}

	errorList.WithObjectID(object.Identity()).WithValue(labels).
		Error(`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`)
}

func objectAPIVersion(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	version := object.Unstructured.GetAPIVersion()

	switch object.Unstructured.GetKind() {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		compareAPIVersion("rbac.authorization.k8s.io/v1", version, object.Identity(), errorList)
	case "Deployment", "DaemonSet", "StatefulSet":
		compareAPIVersion("apps/v1", version, object.Identity(), errorList)
	case "Ingress":
		compareAPIVersion("networking.k8s.io/v1", version, object.Identity(), errorList)
	case "PriorityClass":
		compareAPIVersion("scheduling.k8s.io/v1", version, object.Identity(), errorList)
	case "PodSecurityPolicy":
		compareAPIVersion("policy/v1beta1", version, object.Identity(), errorList)
	case "NetworkPolicy":
		compareAPIVersion("networking.k8s.io/v1", version, object.Identity(), errorList)
	}
}

func compareAPIVersion(wanted, version, objectID string, errorList *errors.LintRuleErrorsList) {
	if version != wanted {
		errorList.WithObjectID(objectID).
			Errorf("Object defined using deprecated api version, wanted %q", wanted)
	}
}

func objectRevisionHistoryLimit(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment); err != nil {
			errorList.WithObjectID(object.Identity()).
				Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

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
			errorList.WithObjectID(object.Identity()).
				Errorf("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		} else if *actualLimit > maxHistoryLimit {
			errorList.WithObjectID(object.Identity()).WithValue(*actualLimit).
				Errorf("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		}
	}
}

func objectPriorityClass(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if !isPriorityClassSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	priorityClass, err := getPriorityClass(object)
	if err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	validatePriorityClass(priorityClass, object, errorList)
}

func isPriorityClassSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet":
		return true
	default:
		return false
	}
}

func validatePriorityClass(priorityClass string, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	switch priorityClass {
	case "":
		errorList.WithObjectID(object.Identity()).WithValue(priorityClass).
			Error("Priority class must not be empty")
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "cluster-critical":
	default:
		errorList.WithObjectID(object.Identity()).WithValue(priorityClass).
			Error("Priority class is not allowed")
	}
}

func getPriorityClass(object storage.StoreObject) (string, error) {
	converter := runtime.DefaultUnstructuredConverter

	var priorityClass string
	var err error

	switch object.Unstructured.GetKind() {
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

func objectSecurityContext(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if !isSecurityContextSupportedKind(object.Unstructured.GetKind()) {
		return
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("GetPodSecurityContext failed: %v", err)

		return
	}

	if securityContext == nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Object's SecurityContext is not defined")
	}

	checkSecurityContextParameters(securityContext, object, errorList)
}

func isSecurityContextSupportedKind(kind string) bool {
	switch kind {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func checkSecurityContextParameters(securityContext *corev1.PodSecurityContext, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if securityContext.RunAsNonRoot == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsNonRoot")
	}

	if securityContext.RunAsUser == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsUser")
	}
	if securityContext.RunAsGroup == nil {
		errorList.WithObjectID(object.Identity()).
			Error("Object's SecurityContext missing parameter RunAsGroup")
	}

	if securityContext.RunAsNonRoot != nil && securityContext.RunAsUser != nil && securityContext.RunAsGroup != nil {
		checkRunAsNonRoot(securityContext, object, errorList)
	}
}

func checkRunAsNonRoot(securityContext *corev1.PodSecurityContext, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	value := fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)

	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			errorList.WithObjectID(object.Identity()).WithValue(value).
				Error("Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)")
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			errorList.WithObjectID(object.Identity()).WithValue(value).
				Error("Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0")
		}
	}
}

func objectServiceTargetPort(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return
	}

	converter := runtime.DefaultUnstructuredConverter

	service := new(corev1.Service)
	if err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service); err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				errorList.WithObjectID(object.Identity()).
					Error("Service port must use an explicit named (non-numeric) target port")

				continue
			}
			errorList.WithObjectID(object.Identity()).WithValue(port.TargetPort.IntVal).
				Error("Service port must use a named (non-numeric) target port")
		}
	}
}

func objectDNSPolicy(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	dnsPolicy, hostNetwork, err := getDNSPolicyAndHostNetwork(object)
	if err != nil {
		errorList.WithObjectID(object.Unstructured.GetName()).
			Errorf("Cannot convert object to %s: %v", object.Unstructured.GetKind(), err)

		return
	}

	validateDNSPolicy(dnsPolicy, hostNetwork, object, errorList)
}

func validateDNSPolicy(dnsPolicy string, hostNetwork bool, object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	if !hostNetwork {
		return
	}

	if dnsPolicy != "ClusterFirstWithHostNet" {
		errorList.WithObjectID(object.Identity()).WithValue(dnsPolicy).
			Error("dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`")
	}
}

func getDNSPolicyAndHostNetwork(object storage.StoreObject) (string, bool, error) { //nolint:gocritic // false positive
	converter := runtime.DefaultUnstructuredConverter

	var dnsPolicy string
	var hostNetwork bool
	var err error

	switch object.Unstructured.GetKind() {
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
