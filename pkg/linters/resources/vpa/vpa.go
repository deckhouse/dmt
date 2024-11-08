/*
Copyright 2021 Flant JSC

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

package vpa

import (
	"fmt"

	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/set"
	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

const (
	ID = "vpa"
)

// ControllerMustHaveVPA fills linting error regarding VPA
func ControllerMustHaveVPA(md *module.Module, objectStore *storage.UnstructuredObjectStore) (result errors.LintRuleErrorsList) {
	vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, errs := parseTargetsAndTolerationGroups(md, objectStore)
	result.Merge(errs)

	for index, object := range objectStore.Storage {
		// Skip non-pod controllers
		if !IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		ok, errs := ensureVPAIsPresent(md, vpaTargets, index, object)
		result.Merge(errs)
		if !ok {
			continue
		}

		// for vpa UpdateMode Off we cannot have container resource policies in vpa object
		if vpaUpdateModes[index] == UpdateModeOff {
			continue
		}

		ok, errs = ensureVPAContainersMatchControllerContainers(md, object, index, vpaContainerNamesMap)
		result.Merge(errs)
		if !ok {
			continue
		}

		result.Merge(ensureTolerations(md, vpaTolerationGroups, index, object))
	}

	return result
}

func IsPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

// parseTargetsAndTolerationGroups resolves target resource indexes
func parseTargetsAndTolerationGroups(md *module.Module, objectStore *storage.UnstructuredObjectStore) (
	vpaTargets map[storage.ResourceIndex]struct{}, vpaTolerationGroups map[storage.ResourceIndex]string,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	vpaUpdateModes map[storage.ResourceIndex]UpdateMode,
	result errors.LintRuleErrorsList,
) {
	vpaTargets = make(map[storage.ResourceIndex]struct{})
	vpaTolerationGroups = make(map[storage.ResourceIndex]string)
	vpaContainerNamesMap = make(map[storage.ResourceIndex]set.Set)
	vpaUpdateModes = make(map[storage.ResourceIndex]UpdateMode)

	for _, object := range objectStore.Storage {
		kind := object.Unstructured.GetKind()

		if kind != "VerticalPodAutoscaler" {
			continue
		}

		result.Merge(fillVPAMaps(md, vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, object))
	}

	return vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, result
}

func fillVPAMaps(
	md *module.Module,
	vpaTargets map[storage.ResourceIndex]struct{},
	vpaTolerationGroups map[storage.ResourceIndex]string,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	vpaUpdateModes map[storage.ResourceIndex]UpdateMode,
	vpa storage.StoreObject,
) (result errors.LintRuleErrorsList) {
	target, ok, errs := parseVPATargetIndex(vpa)
	result.Merge(errs)
	if !ok {
		return
	}

	vpaTargets[target] = struct{}{}

	labels := vpa.Unstructured.GetLabels()
	if label, lok := labels["workload-resource-policy.deckhouse.io"]; lok {
		vpaTolerationGroups[target] = label
	}

	updateMode, vnm, ok, errs := parseVPAResourcePolicyContainers(md, vpa)
	result.Merge(errs)
	if !ok {
		return
	}
	vpaContainerNamesMap[target] = vnm
	vpaUpdateModes[target] = updateMode

	return result
}

// parseVPAResourcePolicyContainers parses VPA containers names in ResourcePolicy and check if minAllowed and maxAllowed for container is set
func parseVPAResourcePolicyContainers(md *module.Module, vpaObject storage.StoreObject) (UpdateMode, set.Set, bool, errors.LintRuleErrorsList) {
	result := errors.LintRuleErrorsList{}
	containers := set.New()

	v := &VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(&vpaObject.Unstructured, v)

	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID, vpaObject.Identity(), md.GetName(), false, "Cannot unmarshal VPA object: %v", err,
		))
		return "", containers, false, result
	}

	updateMode := *v.Spec.UpdatePolicy.UpdateMode
	if updateMode == UpdateModeOff {
		return updateMode, containers, true, result
	}

	if v.Spec.ResourcePolicy == nil || len(v.Spec.ResourcePolicy.ContainerPolicies) == 0 {
		result.Add(errors.NewLintRuleError(
			ID, vpaObject.Identity(), md.GetName(), false, "No VPA specs resourcePolicy.containerPolicies is found for object",
		))
		return updateMode, containers, false, result
	}

	for _, cp := range v.Spec.ResourcePolicy.ContainerPolicies {
		if cp.MinAllowed.Cpu().IsZero() {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "No VPA specs minAllowed.cpu is found for container %s", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		if cp.MinAllowed.Memory().IsZero() {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "No VPA specs minAllowed.memory is found for container %s", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		if cp.MaxAllowed.Cpu().IsZero() {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "No VPA specs maxAllowed.cpu is found for container %s", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		if cp.MaxAllowed.Memory().IsZero() {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "No VPA specs maxAllowed.memory is found for container %s", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		if cp.MinAllowed.Cpu().Cmp(*cp.MaxAllowed.Cpu()) > 0 {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "MinAllowed.cpu for container %s should be less than maxAllowed.cpu", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		if cp.MinAllowed.Memory().Cmp(*cp.MaxAllowed.Memory()) > 0 {
			result.Add(errors.NewLintRuleError(
				ID, vpaObject.Identity(), md.GetName(), false, "MinAllowed.memory for container %s should be less than maxAllowed.memory", cp.ContainerName,
			))
			//return updateMode, containers, false, result
		}

		containers.Add(cp.ContainerName)
	}

	return updateMode, containers, true, result
}

// parseVPATargetIndex parses VPA target resource index, writes to the passed struct pointer
func parseVPATargetIndex(vpaObject storage.StoreObject) (target storage.ResourceIndex, ok bool, result errors.LintRuleErrorsList) {
	specs, ok := vpaObject.Unstructured.Object["spec"].(map[string]any)
	if !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			vpaObject.Identity(),
			"",
			false,
			"No VPA specs is found for object",
		))
		return target, false, result
	}

	targetRef, ok := specs["targetRef"].(map[string]any)
	if !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			vpaObject.Identity(),
			"",
			false,
			"No VPA specs targetRef is found for object",
		))
		return target, false, result
	}

	target.Namespace = vpaObject.Unstructured.GetNamespace()
	target.Name = targetRef["name"].(string)
	target.Kind = targetRef["kind"].(string)

	return target, true, result
}

// ensureVPAContainersMatchControllerContainers verifies VPA container names in resourcePolicy match corresponding controller container names
func ensureVPAContainersMatchControllerContainers(
	md *module.Module,
	object storage.StoreObject,
	index storage.ResourceIndex,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
) (bool, errors.LintRuleErrorsList) {
	result := errors.LintRuleErrorsList{}
	vpaContainerNames, ok := vpaContainerNamesMap[index]
	if !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			false,
			"Getting vpa containers name list for the object failed: %v",
			index,
		))
		return false, result
	}

	containers, err := object.GetContainers()
	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			false,
			"Getting containers list for the object failed: %v",
			err,
		))
		return false, result
	}

	containerNames := set.New()
	for i := range containers {
		containerNames.Add(containers[i].Name)
	}

	for k := range containerNames {
		if !vpaContainerNames.Has(k) {
			result.Add(errors.NewLintRuleError(
				ID,
				fmt.Sprintf("%s ; container = %s", object.Identity(), k),
				md.GetName(),
				false,
				"The container should have corresponding VPA resourcePolicy entry",
			))
		}
	}

	for k := range vpaContainerNames {
		if !containerNames.Has(k) {
			result.Add(errors.NewLintRuleError(
				ID,
				object.Identity(),
				md.GetName(),
				false,
				"VPA has resourcePolicy for container %s, but the controller does not have corresponding container resource entry", k,
			))
		}
	}

	return true, result
}

// returns true if linting passed, otherwise returns false
func ensureTolerations(
	md *module.Module,
	vpaTolerationGroups map[storage.ResourceIndex]string,
	index storage.ResourceIndex,
	object storage.StoreObject,
) (result errors.LintRuleErrorsList) {
	tolerations, err := getTolerationsList(object)

	if err != nil {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			false,
			"Get tolerations list for object failed: %v",
			err,
		))
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
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			workloadLabelValue,
			`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource not found`,
		))
	}

	if !isTolerationFound && workloadLabelValue != "" {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			workloadLabelValue,
			`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource found, but tolerations is not right`,
		))
	}

	return result
}

// returns true if linting passed, otherwise returns false
func ensureVPAIsPresent(
	md *module.Module,
	vpaTargets map[storage.ResourceIndex]struct{},
	index storage.ResourceIndex,
	object storage.StoreObject,
) (bool, errors.LintRuleErrorsList) {
	result := errors.LintRuleErrorsList{}
	_, ok := vpaTargets[index]
	if !ok {
		result.Add(errors.NewLintRuleError(
			ID,
			object.Identity(),
			md.GetName(),
			nil,
			"No VPA is found for object"),
		)
	}

	return ok, result
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
