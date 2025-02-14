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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	PDBRuleName = "pdb"
)

func NewPDBRule(excludeRules []pkg.KindRuleExclude) *PDBRule {
	return &PDBRule{
		RuleMeta: pkg.RuleMeta{
			Name: PDBRuleName,
		},
		KindRule: pkg.KindRule{
			ExcludeRules: excludeRules,
		},
	}
}

type PDBRule struct {
	pkg.RuleMeta
	pkg.KindRule
}

type nsLabelSelector struct {
	namespace string
	selector  labels.Selector
}

func (s *nsLabelSelector) Matches(namespace string, labelSet labels.Set) bool {
	return s.namespace == namespace && s.selector.Matches(labelSet)
}

// controllerMustHavePDB adds linting errors if there are pods from controllers which are not covered (except DaemonSets)
// by a PodDisruptionBudget
func (r *PDBRule) ControllerMustHavePDB(md *module.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	pdbSelectors := collectPDBSelectors(md, errorList)

	for _, object := range md.GetObjectStore().Storage {
		if !isPodController(object.Unstructured.GetKind()) {
			continue
		}

		if !r.Enabled(object.Unstructured.GetKind(), object.Unstructured.GetName()) {
			// TODO: add metrics
			continue
		}

		if len(pdbSelectors) == 0 {
			errorList.WithObjectID(object.Identity()).
				WithFilePath(object.ShortPath()).
				Error("No PodDisruptionBudget found for controller")
			return
		}

		ensurePDBIsPresent(pdbSelectors, object, errorList)
	}
}

// collectPDBSelectors collects selectors for matching pods
func collectPDBSelectors(md *module.Module, errorList *errors.LintRuleErrorsList) []nsLabelSelector {
	var selectors []nsLabelSelector

	for _, object := range md.GetObjectStore().Storage {
		if object.Unstructured.GetKind() != "PodDisruptionBudget" {
			continue
		}

		labelSelector := parsePDBSelector(object, errorList)

		sel := nsLabelSelector{
			namespace: object.Unstructured.GetNamespace(),
			selector:  labelSelector,
		}

		selectors = append(selectors, sel)
	}

	return selectors
}

// ensurePDBIsPresent returns true if there is a PDB controlling pods from the pod contoller
// PDB is assumed to be present, since the PDB check goes after PDB check.
func ensurePDBIsPresent(selectors []nsLabelSelector, podController storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	errorListObj := errorList.WithObjectID(podController.Identity()).WithFilePath(podController.ShortPath())

	podLabels, err := parsePodControllerLabels(podController)
	if err != nil {
		errorListObj.Errorf("Cannot parse pod controller: %s", err)
	}

	podNamespace := podController.Unstructured.GetNamespace()
	podLabelsSet := labels.Set(podLabels)

	for _, sel := range selectors {
		if sel.Matches(podNamespace, podLabelsSet) {
			return
		}
	}

	errorListObj.WithValue(podLabelsSet).
		Error("No PodDisruptionBudget matches pod labels of the controller")
}

func parsePDBSelector(pdbObj storage.StoreObject, errorList *errors.LintRuleErrorsList) labels.Selector {
	content := pdbObj.Unstructured.UnstructuredContent()
	converter := runtime.DefaultUnstructuredConverter

	errorListObj := errorList.WithObjectID(pdbObj.Identity())

	pdb := &policyv1.PodDisruptionBudget{}
	err := converter.FromUnstructured(content, pdb)
	if err != nil {
		errorListObj.Errorf("Cannot parse PodDisruptionBudget: %s", err)

		return nil
	}

	sel, err := v1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		errorListObj.Errorf("Cannot parse label selector: %s", err)

		return nil
	}

	if pdb.Annotations["helm.sh/hook"] != "" || pdb.Annotations["helm.sh/hook-delete-policy"] != "" {
		errorListObj.Error("PDB must have no helm hook annotations")

		return nil
	}

	return sel
}

func parsePodControllerLabels(object storage.StoreObject) (map[string]string, error) {
	content := object.Unstructured.UnstructuredContent()
	converter := runtime.DefaultUnstructuredConverter
	kind := object.Unstructured.GetKind()

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(content, deployment)
		if err != nil {
			return nil, err
		}

		return deployment.Spec.Template.Labels, nil

	case "DaemonSet":
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(content, daemonSet)
		if err != nil {
			return nil, err
		}

		return daemonSet.Spec.Template.Labels, nil

	case "StatefulSet":
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(content, statefulSet)
		if err != nil {
			return nil, err
		}

		return statefulSet.Spec.Template.Labels, nil

	default:
		return nil, fmt.Errorf("object of kind %s is not a pod controller", kind)
	}
}

func isPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}
