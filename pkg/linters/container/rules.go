package container

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func applyContainerRules(m *module.Module, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())

	objectRules := []func(string, storage.StoreObject) *errors.LintRuleErrorsList{
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
		result.Merge(rule(m.GetName(), object))
	}

	containers, err := object.GetAllContainers()
	if err != nil {
		result.WithValue(err).Add("Cannot get containers from object: %s", object.Identity())
		return result
	}

	if len(containers) == 0 {
		return result
	}

	containerRules := []func(string, storage.StoreObject, []v1.Container) *errors.LintRuleErrorsList{
		containerNameDuplicates,
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		containerStorageEphemeral,
		containerSecurityContext,
		containerPorts,
		objectReadOnlyRootFilesystem,
		objectHostNetworkPorts,
	}

	for _, rule := range containerRules {
		result.Merge(rule(m.GetName(), object, containers))
	}

	return result
}

func containersImagePullPolicy(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	if object.Unstructured.GetNamespace() == "d8-system" && object.Unstructured.GetKind() == "Deployment" && object.Unstructured.GetName() == "deckhouse" {
		return checkImagePullPolicyAlways(md, object, containers)
	}
	return containerImagePullPolicyIfNotPresent(md, object, containers)
}

func checkImagePullPolicyAlways(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	c := containers[0]
	if c.ImagePullPolicy != v1.PullAlways {
		return errors.NewLinterRuleList(ID, md).WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "Always"`)
	}
	return nil
}

func containerNameDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	return checkForDuplicates(md, object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name")
}

func containerEnvVariablesDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		if err := checkForDuplicates(md, object, c.Env, func(e v1.EnvVar) string { return e.Name }, "Container has two env variables with same name"); err != nil {
			return err
		}
	}
	return nil
}

func checkForDuplicates[T any](md string, object storage.StoreObject, items []T, keyFunc func(T) string, errMsg string) *errors.LintRuleErrorsList {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			return errors.NewLinterRuleList(ID, md).WithObjectID(object.Identity()).
				Add("%s", errMsg)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func shouldSkipModuleContainer(md, container string) bool {
	for _, line := range Cfg.SkipContainers {
		els := strings.Split(line, ":")
		if len(els) != 2 {
			continue
		}
		moduleName := strings.TrimSpace(els[0])
		containerName := strings.TrimSpace(els[1])

		checkContainer := container == containerName
		subString := strings.Trim(containerName, "*")
		if len(subString) != len(containerName) {
			checkContainer = strings.Contains(container, subString)
		}

		if md == moduleName && checkContainer {
			return true
		}
	}

	return false
}

func containerImageDigestCheck(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity() + "; container = " + c.Name).Add("Cannot parse repository from image")
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("Cannot parse repository from image: %s", c.Image)
		}

		if repo.Name() != defaultRegistry {
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("All images must be deployed from the same default registry: %s current: %s",
					defaultRegistry,
					repo.RepositoryStr())
		}
	}
	return nil
}

func containerImagePullPolicyIfNotPresent(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return errors.NewLinterRuleList(ID, md).
			WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
	return nil
}

func containerStorageEphemeral(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity() + "; container = " + c.Name).
				Add("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
	return nil
}

func containerSecurityContext(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		if c.SecurityContext == nil {
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity() + "; container = " + c.Name).
				Add("Container SecurityContext is not defined")
		}
	}
	return nil
}

func containerPorts(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				return errors.NewLinterRuleList(ID, md).
					WithObjectID(object.Identity() + "; container = " + c.Name).
					WithValue(p.ContainerPort).
					Add("Container uses port <= 1024")
			}
		}
	}
	return nil
}

func objectReadOnlyRootFilesystem(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, md)
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return result
	}

	for i := range containers {
		c := &containers[i]
		if c.VolumeMounts == nil {
			continue
		}
		if c.SecurityContext == nil {
			result.WithObjectID(object.Identity()).Add("Container's SecurityContext is missing")
			continue
		}
		if c.SecurityContext.ReadOnlyRootFilesystem == nil {
			result.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
				Add("Container's SecurityContext missing parameter ReadOnlyRootFilesystem")
			continue
		}
		if !*c.SecurityContext.ReadOnlyRootFilesystem {
			result.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).Add(
				"Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`",
			)
		}
	}

	return result
}

func objectHostNetworkPorts(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, md)
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return result
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		return result.WithObjectID(object.Identity()).Add("IsHostNetwork failed: %v", err)
	}

	for i := range containers {
		for _, p := range containers[i].Ports {
			if hostNetworkUsed && (p.ContainerPort < 4200 || p.ContainerPort >= 4300) {
				result.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
					WithValue(p.ContainerPort).
					Add("Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]")
			}
			if p.HostPort != 0 && (p.HostPort < 4200 || p.HostPort >= 4300) {
				result.WithObjectID(object.Identity() + " ; container = " + containers[i].Name).
					WithValue(p.HostPort).
					Add("Container uses hostPort that doesn't fit the range [4200,4299]")
			}
		}
	}

	return result
}

func objectRecommendedLabels(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		result.WithObjectID(object.Identity()).WithValue(labels).
			Add(`Object does not have the label "module"`)
	}
	if _, ok := labels["heritage"]; !ok {
		result.WithObjectID(object.Identity()).WithValue(labels).
			Add(`Object does not have the label "heritage"`)
	}

	return result
}

func namespaceLabels(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
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

	result.WithObjectID(object.Identity()).WithValue(labels).
		Add(`Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"`)

	return result
}

func newAPIVersionError(name, wanted, version, objectID string) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	if version != wanted {
		result.WithObjectID(objectID).Add(
			"Object defined using deprecated api version, wanted %q", wanted,
		)
	}
	return result
}

func objectAPIVersion(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	kind := object.Unstructured.GetKind()
	version := object.Unstructured.GetAPIVersion()

	switch kind {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		result.Merge(newAPIVersionError(name, "rbac.authorization.k8s.io/v1", version, object.Identity()))
	case "Deployment", "DaemonSet", "StatefulSet":
		result.Merge(newAPIVersionError(name, "apps/v1", version, object.Identity()))
	case "Ingress":
		result.Merge(newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity()))
	case "PriorityClass":
		result.Merge(newAPIVersionError(name, "scheduling.k8s.io/v1", version, object.Identity()))
	case "PodSecurityPolicy":
		result.Merge(newAPIVersionError(name, "policy/v1beta1", version, object.Identity()))
	case "NetworkPolicy":
		result.Merge(newAPIVersionError(name, "networking.k8s.io/v1", version, object.Identity()))
	}

	return result
}

func objectRevisionHistoryLimit(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return result.WithObjectID(object.Unstructured.GetName()).Add(
				"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
			)
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
			result.WithObjectID(object.Identity()).Add(
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		} else if *actualLimit > maxHistoryLimit {
			result.WithObjectID(object.Identity()).WithValue(*actualLimit).
				Add("Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit)
		}
	}

	return result
}

func objectPriorityClass(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	if !isPriorityClassSupportedKind(object.Unstructured.GetKind()) {
		return result
	}

	priorityClass, err := getPriorityClass(object)
	if err != nil {
		return result.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
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

func validatePriorityClass(priorityClass, _ string, object storage.StoreObject, result *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	switch priorityClass {
	case "":
		result.WithObjectID(object.Identity()).WithValue(priorityClass).
			Add("Priority class must not be empty")
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "cluster-critical":
	default:
		result.WithObjectID(object.Identity()).WithValue(priorityClass).
			Add("Priority class is not allowed")
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

func objectSecurityContext(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	if !isSupportedKind(object.Unstructured.GetKind()) {
		return result
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		return result.WithObjectID(object.Identity()).Add("GetPodSecurityContext failed: %v", err)
	}

	if securityContext == nil {
		return result.WithObjectID(object.Identity()).Add("Object's SecurityContext is not defined")
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

func checkSecurityContextParameters(securityContext *v1.PodSecurityContext, result *errors.LintRuleErrorsList, object storage.StoreObject, name string) {
	if securityContext.RunAsNonRoot == nil {
		result.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsNonRoot")
	}

	if securityContext.RunAsUser == nil {
		result.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsUser")
	}
	if securityContext.RunAsGroup == nil {
		result.WithObjectID(object.Identity()).Add("Object's SecurityContext missing parameter RunAsGroup")
	}

	if securityContext.RunAsNonRoot != nil && securityContext.RunAsUser != nil && securityContext.RunAsGroup != nil {
		checkRunAsNonRoot(securityContext, result, object, name)
	}
}

func checkRunAsNonRoot(securityContext *v1.PodSecurityContext, result *errors.LintRuleErrorsList, object storage.StoreObject, _ string) {
	switch *securityContext.RunAsNonRoot {
	case true:
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			result.WithObjectID(object.Identity()).
				WithValue(fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)).
				Add("Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)")
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			result.WithObjectID(object.Identity()).
				WithValue(fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup)).
				Add("Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0")
		}
	}
}

func objectServiceTargetPort(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	switch object.Unstructured.GetKind() {
	case "Service":
	default:
		return result
	}

	converter := runtime.DefaultUnstructuredConverter
	service := new(v1.Service)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), service)
	if err != nil {
		return result.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
	}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.Type == intstr.Int {
			if port.TargetPort.IntVal == 0 {
				result.WithObjectID(object.Identity()).Add(
					"Service port must use an explicit named (non-numeric) target port",
				)

				continue
			}
			result.WithObjectID(object.Identity()).WithValue(port.TargetPort.IntVal).
				Add("Service port must use a named (non-numeric) target port")
		}
	}

	return result
}

func objectDNSPolicy(name string, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, name)
	dnsPolicy, hostNetwork, err := getDNSPolicyAndHostNetwork(object)
	if err != nil {
		return result.WithObjectID(object.Unstructured.GetName()).Add(
			"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
		)
	}

	return validateDNSPolicy(dnsPolicy, hostNetwork, name, object, result)
}

func validateDNSPolicy(dnsPolicy string, hostNetwork bool, _ string, object storage.StoreObject, result *errors.LintRuleErrorsList) *errors.LintRuleErrorsList {
	if !hostNetwork {
		return result
	}

	if dnsPolicy != "ClusterFirstWithHostNet" {
		result.WithObjectID(object.Identity()).WithValue(dnsPolicy).
			Add("dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`")
	}

	return result
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
