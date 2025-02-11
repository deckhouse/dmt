package vpa

import (
	"fmt"
	"slices"

	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	ID = "vpa"
)

var SkipVPAChecks []string

// ControllerMustHaveVPA fills linting error regarding VPA
func ControllerMustHaveVPA(md *module.Module, lintError *errors.Error) {
	if slices.Contains(SkipVPAChecks, md.GetNamespace()+":"+md.GetName()) {
		return
	}

	vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes := parseTargetsAndTolerationGroups(md, lintError)

	for index, object := range md.GetObjectStore().Storage {
		// Skip non-pod controllers
		if !IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		ok := ensureVPAIsPresent(md, vpaTargets, index, object, lintError)
		if !ok {
			continue
		}

		// for vpa UpdateMode Off we cannot have container resource policies in vpa object
		if vpaUpdateModes[index] == UpdateModeOff {
			continue
		}

		ok = ensureVPAContainersMatchControllerContainers(md, object, index, vpaContainerNamesMap, lintError)
		if !ok {
			continue
		}

		ensureTolerations(md, vpaTolerationGroups, index, object, lintError)
	}
}

func IsPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

// parseTargetsAndTolerationGroups resolves target resource indexes
//
//nolint:gocritic // false positive
func parseTargetsAndTolerationGroups(md *module.Module, lintError *errors.Error) (
	map[storage.ResourceIndex]struct{}, map[storage.ResourceIndex]string,
	map[storage.ResourceIndex]set.Set,
	map[storage.ResourceIndex]UpdateMode,
) {
	vpaTargets := make(map[storage.ResourceIndex]struct{})
	vpaTolerationGroups := make(map[storage.ResourceIndex]string)
	vpaContainerNamesMap := make(map[storage.ResourceIndex]set.Set)
	vpaUpdateModes := make(map[storage.ResourceIndex]UpdateMode)

	for _, object := range md.GetObjectStore().Storage {
		kind := object.Unstructured.GetKind()

		if kind != "VerticalPodAutoscaler" {
			continue
		}

		fillVPAMaps(md, vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, object, lintError)
	}

	return vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes
}

func fillVPAMaps(
	md *module.Module,
	vpaTargets map[storage.ResourceIndex]struct{},
	vpaTolerationGroups map[storage.ResourceIndex]string,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	vpaUpdateModes map[storage.ResourceIndex]UpdateMode,
	vpa storage.StoreObject,
	lintError *errors.Error,
) {
	target, ok := parseVPATargetIndex(md.GetName(), vpa, lintError)
	if !ok {
		return
	}

	vpaTargets[target] = struct{}{}

	labels := vpa.Unstructured.GetLabels()
	if label, lok := labels["workload-resource-policy.deckhouse.io"]; lok {
		vpaTolerationGroups[target] = label
	}

	updateMode, vnm, ok := parseVPAResourcePolicyContainers(md, vpa, lintError)
	if !ok {
		return
	}
	vpaContainerNamesMap[target] = vnm
	vpaUpdateModes[target] = updateMode

	return
}

// parseVPAResourcePolicyContainers parses VPA containers names in ResourcePolicy and check if minAllowed and maxAllowed for container is set
func parseVPAResourcePolicyContainers(md *module.Module, vpaObject storage.StoreObject, lintError *errors.Error) (UpdateMode, set.Set, bool) {
	containers := set.New()

	v := &VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(&vpaObject.Unstructured, v)

	if err != nil {
		lintError.WithObjectID(vpaObject.Identity()).Add("Cannot unmarshal VPA object: %v", err)
		return "", containers, false
	}

	updateMode := *v.Spec.UpdatePolicy.UpdateMode
	if updateMode == UpdateModeOff {
		return updateMode, containers, true
	}

	if v.Spec.ResourcePolicy == nil || len(v.Spec.ResourcePolicy.ContainerPolicies) == 0 {
		lintError.WithObjectID(vpaObject.Identity()).
			Add("No VPA specs resourcePolicy.containerPolicies is found for object")
		return updateMode, containers, false
	}

	for _, cp := range v.Spec.ResourcePolicy.ContainerPolicies {
		if cp.MinAllowed.Cpu().IsZero() {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("No VPA specs minAllowed.cpu is found for container %s", cp.ContainerName)
		}

		if cp.MinAllowed.Memory().IsZero() {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("No VPA specs minAllowed.memory is found for container %s", cp.ContainerName)
		}

		if cp.MaxAllowed.Cpu().IsZero() {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("No VPA specs maxAllowed.cpu is found for container %s", cp.ContainerName)
		}

		if cp.MaxAllowed.Memory().IsZero() {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("No VPA specs maxAllowed.memory is found for container %s", cp.ContainerName)
		}

		if cp.MinAllowed.Cpu().Cmp(*cp.MaxAllowed.Cpu()) > 0 {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("MinAllowed.cpu for container %s should be less than maxAllowed.cpu", cp.ContainerName)
		}

		if cp.MinAllowed.Memory().Cmp(*cp.MaxAllowed.Memory()) > 0 {
			lintError.WithObjectID(vpaObject.Identity()).
				Add("MinAllowed.memory for container %s should be less than maxAllowed.memory", cp.ContainerName)
		}

		containers.Add(cp.ContainerName)
	}

	return updateMode, containers, true
}

// parseVPATargetIndex parses VPA target resource index, writes to the passed struct pointer
func parseVPATargetIndex(name string, vpaObject storage.StoreObject, lintError *errors.Error) (storage.ResourceIndex, bool) {
	target := storage.ResourceIndex{}
	specs, ok := vpaObject.Unstructured.Object["spec"].(map[string]any)
	if !ok {
		lintError.WithObjectID(
			vpaObject.Identity()).Add("No VPA specs is found for object")
		return target, false
	}

	targetRef, ok := specs["targetRef"].(map[string]any)
	if !ok {
		lintError.WithObjectID(
			vpaObject.Identity()).Add("No VPA specs targetRef is found for object")
		return target, false
	}

	target.Namespace = vpaObject.Unstructured.GetNamespace()
	target.Name = targetRef["name"].(string)
	target.Kind = targetRef["kind"].(string)

	return target, true
}

// ensureVPAContainersMatchControllerContainers verifies VPA container names in resourcePolicy match corresponding controller container names
func ensureVPAContainersMatchControllerContainers(
	md *module.Module,
	object storage.StoreObject,
	index storage.ResourceIndex,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	lintError *errors.Error,
) bool {
	vpaContainerNames, ok := vpaContainerNamesMap[index]
	if !ok {
		lintError.WithObjectID(object.Identity()).Add(
			"Getting vpa containers name list for the object failed: %v",
			index,
		)
		return false
	}

	containers, err := object.GetContainers()
	if err != nil {
		lintError.WithObjectID(
			object.Identity()).Add(
			"Getting containers list for the object failed: %v",
			err,
		)
		return false
	}

	containerNames := set.New()
	for i := range containers {
		containerNames.Add(containers[i].Name)
	}

	for k := range containerNames {
		if !vpaContainerNames.Has(k) {
			lintError.WithObjectID(
				fmt.Sprintf("%s ; container = %s", object.Identity(), k)).Add(
				"The container should have corresponding VPA resourcePolicy entry",
			)
		}
	}

	for k := range vpaContainerNames {
		if !containerNames.Has(k) {
			lintError.WithObjectID(
				object.Identity()).Add(
				"VPA has resourcePolicy for container %s, but the controller does not have corresponding container resource entry", k,
			)
		}
	}

	return true
}

// returns true if linting passed, otherwise returns false
func ensureTolerations(
	md *module.Module,
	vpaTolerationGroups map[storage.ResourceIndex]string,
	index storage.ResourceIndex,
	object storage.StoreObject,
	lintError *errors.Error,
) {
	tolerations, err := getTolerationsList(object)

	if err != nil {
		lintError.WithObjectID(
			object.Identity()).Add(
			"Get tolerations list for object failed: %v",
			err,
		)
	}

	isTolerationFound := false
	for _, toleration := range tolerations {
		if toleration.Key == "node-role.kubernetes.io/master" || toleration.Key == "node-role.kubernetes.io/control-plane" || (toleration.Key == "" && toleration.Operator == "Exists") {
			isTolerationFound = true
			break
		}
	}

	workloadLabelValue := vpaTolerationGroups[index]
	if isTolerationFound && workloadLabelValue != "every-node" && workloadLabelValue != "master" {
		lintError.WithObjectID(
			object.Identity()).WithValue(workloadLabelValue).
			Add(`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource not found`)
	}

	if !isTolerationFound && workloadLabelValue != "" {
		lintError.WithObjectID(
			object.Identity()).WithValue(workloadLabelValue).
			Add(`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource found, but tolerations is not right`)
	}
}

// returns true if linting passed, otherwise returns false
func ensureVPAIsPresent(
	md *module.Module,
	vpaTargets map[storage.ResourceIndex]struct{},
	index storage.ResourceIndex,
	object storage.StoreObject,
	lintError *errors.Error,
) bool {
	_, ok := vpaTargets[index]
	if !ok {
		lintError.WithObjectID(object.Identity()).Add("No VPA is found for object")
	}

	return ok
}

func getTolerationsList(object storage.StoreObject) ([]v1.Toleration, error) {
	var tolerations []v1.Toleration
	converter := runtime.DefaultUnstructuredConverter

	switch object.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return nil, err
		}
		tolerations = deployment.Spec.Template.Spec.Tolerations

	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return nil, err
		}
		tolerations = daemonset.Spec.Template.Spec.Tolerations

	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return nil, err
		}
		tolerations = statefulset.Spec.Template.Spec.Tolerations
	}

	return tolerations, nil
}
