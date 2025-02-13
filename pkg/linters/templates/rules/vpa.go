package rules

import (
	"fmt"

	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	VPAAbsentRuleName = "vpa-absent"
)

func NewVPAAbsentRule(excludeRules []pkg.TargetRefRuleExclude) *VPAAbsentRule {
	return &VPAAbsentRule{
		RuleMeta: pkg.RuleMeta{
			Name: VPAAbsentRuleName,
		},
		TargetRefRule: pkg.TargetRefRule{
			ExcludeRules: excludeRules,
		},
	}
}

type VPAAbsentRule struct {
	pkg.RuleMeta
	pkg.TargetRefRule
}

// controllerMustHaveVPA fills linting error regarding VPA
func (r *VPAAbsentRule) ControllerMustHaveVPA(md *module.Module, tr *TolerationsRule, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes := parseTargetsAndTolerationGroups(md, errorList)

	for index, object := range md.GetObjectStore().Storage {
		// Skip non-pod controllers
		if !IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		// spec:
		//	targetRef:
		//	  apiVersion: apps/v1
		//	  kind: Deployment
		//	  name: nginx
		kind, foundKind, err := unstructured.NestedString(object.Unstructured.Object, "spec", "targetRef", "kind")
		if err != nil {
			errorList.Error("parse target ref kind")
			return
		}
		name, foundName, err := unstructured.NestedString(object.Unstructured.Object, "spec", "targetRef", "name")
		if err != nil {
			errorList.Error("parse target ref name")
			return
		}
		if !foundKind || !foundName {
			errorList.Error("not found target ref")
			return
		}

		if !r.Enabled(kind, name) {
			// TODO: add metrics
			return
		}

		ok := ensureVPAIsPresent(vpaTargets, index, errorList.WithObjectID(object.Identity()))
		if !ok {
			continue
		}

		// for vpa UpdateMode Off we cannot have container resource policies in vpa object
		if vpaUpdateModes[index] == UpdateModeOff {
			continue
		}

		ok = ensureVPAContainersMatchControllerContainers(object, index, vpaContainerNamesMap, errorList)
		if !ok {
			continue
		}

		tr.EnsureTolerations(vpaTolerationGroups, index, object, errorList)
	}
}

func IsPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

// parseTargetsAndTolerationGroups resolves target resource indexes
//
//nolint:gocritic // false positive
func parseTargetsAndTolerationGroups(md *module.Module, errorList *errors.LintRuleErrorsList) (
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

		fillVPAMaps(vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, object, errorList)
	}

	return vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes
}

func fillVPAMaps(
	vpaTargets map[storage.ResourceIndex]struct{},
	vpaTolerationGroups map[storage.ResourceIndex]string,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	vpaUpdateModes map[storage.ResourceIndex]UpdateMode,
	vpa storage.StoreObject,
	errorList *errors.LintRuleErrorsList,
) {
	target, ok := parseVPATargetIndex(vpa, errorList)
	if !ok {
		return
	}

	vpaTargets[target] = struct{}{}

	labels := vpa.Unstructured.GetLabels()
	if label, lok := labels["workload-resource-policy.deckhouse.io"]; lok {
		vpaTolerationGroups[target] = label
	}

	updateMode, vnm, ok := parseVPAResourcePolicyContainers(vpa, errorList)
	if !ok {
		return
	}

	vpaContainerNamesMap[target] = vnm
	vpaUpdateModes[target] = updateMode
}

// parseVPAResourcePolicyContainers parses VPA containers names in ResourcePolicy and check if minAllowed and maxAllowed for container is set
func parseVPAResourcePolicyContainers(vpaObject storage.StoreObject, errorList *errors.LintRuleErrorsList) (UpdateMode, set.Set, bool) {
	errorListObj := errorList.WithObjectID(vpaObject.Identity())

	containers := set.New()

	v := &VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(&vpaObject.Unstructured, v)

	if err != nil {
		errorListObj.Errorf("Cannot unmarshal VPA object: %v", err)

		return "", containers, false
	}

	updateMode := *v.Spec.UpdatePolicy.UpdateMode
	if updateMode == UpdateModeOff {
		return updateMode, containers, true
	}

	if v.Spec.ResourcePolicy == nil || len(v.Spec.ResourcePolicy.ContainerPolicies) == 0 {
		errorListObj.Error("No VPA specs resourcePolicy.containerPolicies is found for object")

		return updateMode, containers, false
	}

	for _, cp := range v.Spec.ResourcePolicy.ContainerPolicies {
		if cp.MinAllowed.Cpu().IsZero() {
			errorListObj.Errorf("No VPA specs minAllowed.cpu is found for container %s", cp.ContainerName)
		}

		if cp.MinAllowed.Memory().IsZero() {
			errorListObj.Errorf("No VPA specs minAllowed.memory is found for container %s", cp.ContainerName)
		}

		if cp.MaxAllowed.Cpu().IsZero() {
			errorListObj.Errorf("No VPA specs maxAllowed.cpu is found for container %s", cp.ContainerName)
		}

		if cp.MaxAllowed.Memory().IsZero() {
			errorListObj.Errorf("No VPA specs maxAllowed.memory is found for container %s", cp.ContainerName)
		}

		if cp.MinAllowed.Cpu().Cmp(*cp.MaxAllowed.Cpu()) > 0 {
			errorListObj.Errorf("MinAllowed.cpu for container %s should be less than maxAllowed.cpu", cp.ContainerName)
		}

		if cp.MinAllowed.Memory().Cmp(*cp.MaxAllowed.Memory()) > 0 {
			errorListObj.Errorf("MinAllowed.memory for container %s should be less than maxAllowed.memory", cp.ContainerName)
		}

		containers.Add(cp.ContainerName)
	}

	return updateMode, containers, true
}

// parseVPATargetIndex parses VPA target resource index, writes to the passed struct pointer
func parseVPATargetIndex(vpaObject storage.StoreObject, errorList *errors.LintRuleErrorsList) (storage.ResourceIndex, bool) {
	errorListObj := errorList.WithObjectID(vpaObject.Identity())

	target := storage.ResourceIndex{}
	specs, ok := vpaObject.Unstructured.Object["spec"].(map[string]any)
	if !ok {
		errorListObj.Error("No VPA specs is found for object")

		return target, false
	}

	targetRef, ok := specs["targetRef"].(map[string]any)
	if !ok {
		errorListObj.Error("No VPA specs targetRef is found for object")

		return target, false
	}

	target.Namespace = vpaObject.Unstructured.GetNamespace()
	target.Name = targetRef["name"].(string)
	target.Kind = targetRef["kind"].(string)

	return target, true
}

// ensureVPAContainersMatchControllerContainers verifies VPA container names in resourcePolicy match corresponding controller container names
func ensureVPAContainersMatchControllerContainers(
	object storage.StoreObject,
	index storage.ResourceIndex,
	vpaContainerNamesMap map[storage.ResourceIndex]set.Set,
	errorList *errors.LintRuleErrorsList,
) bool {
	vpaContainerNames, ok := vpaContainerNamesMap[index]
	if !ok {
		errorList.WithObjectID(object.Identity()).
			Errorf("Getting vpa containers name list for the object failed: %v", index)

		return false
	}

	containers, err := object.GetContainers()
	if err != nil {
		errorList.WithObjectID(object.Identity()).
			Errorf("Getting containers list for the object failed: %s", err)

		return false
	}

	containerNames := set.New()
	for i := range containers {
		containerNames.Add(containers[i].Name)
	}

	for k := range containerNames {
		if !vpaContainerNames.Has(k) {
			errorList.WithObjectID(fmt.Sprintf("%s ; container = %s", object.Identity(), k)).
				Error("The container should have corresponding VPA resourcePolicy entry")
		}
	}

	for k := range vpaContainerNames {
		if !containerNames.Has(k) {
			errorList.WithObjectID(object.Identity()).
				Errorf("VPA has resourcePolicy for container %s, but the controller does not have corresponding container resource entry", k)
		}
	}

	return true
}

// returns true if linting passed, otherwise returns false
func ensureVPAIsPresent(
	vpaTargets map[storage.ResourceIndex]struct{},
	index storage.ResourceIndex,
	errorList *errors.LintRuleErrorsList,
) bool {
	_, ok := vpaTargets[index]
	if !ok {
		errorList.Error("No VPA is found for object")
	}

	return ok
}
