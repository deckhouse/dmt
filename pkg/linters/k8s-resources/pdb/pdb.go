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

package pdb

import (
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/k8s-resources/vpa"
)

type nsLabelSelector struct {
	namespace string
	selector  labels.Selector
}

var SkipPDBChecks []string

func (s *nsLabelSelector) Matches(namespace string, labelSet labels.Set) bool {
	return s.namespace == namespace && s.selector.Matches(labelSet)
}

// ControllerMustHavePDB adds linting errors if there are pods from controllers which are not covered (except DaemonSets)
// by a PodDisruptionBudget
func ControllerMustHavePDB(md *module.Module, lintError *errors.Error) {
	if slices.Contains(SkipPDBChecks, md.GetNamespace()+":"+md.GetName()) {
		return
	}

	pdbSelectors := collectPDBSelectors(md, lintError)

	for _, object := range md.GetObjectStore().Storage {
		if !vpa.IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		if isPodControllerDaemonSet(object.Unstructured.GetKind()) {
			continue
		}

		ensurePDBIsPresent(pdbSelectors, object, lintError)
	}
}

func isPodControllerDaemonSet(kind string) bool {
	return kind == "DaemonSet"
}

// DaemonSetMustNotHavePDB adds linting errors if there are pods from DaemonSets which are covered
// by a PodDisruptionBudget
func DaemonSetMustNotHavePDB(moduleName *module.Module, lintError *errors.Error) {
	if slices.Contains(SkipPDBChecks, moduleName.GetNamespace()+":"+moduleName.GetName()) {
		return
	}

	pdbSelectors := collectPDBSelectors(moduleName, lintError)

	for _, object := range moduleName.GetObjectStore().Storage {
		if !vpa.IsPodController(object.Unstructured.GetKind()) {
			continue
		}

		if !isPodControllerDaemonSet(object.Unstructured.GetKind()) {
			continue
		}

		ensurePDBIsNotPresent(pdbSelectors, object, lintError)
	}
}

// collectPDBSelectors collects selectors for matching pods
func collectPDBSelectors(moduleName *module.Module, lintError *errors.Error) []nsLabelSelector {
	var selectors []nsLabelSelector
	for _, object := range moduleName.GetObjectStore().Storage {
		if object.Unstructured.GetKind() != "PodDisruptionBudget" {
			continue
		}

		labelSelector := parsePDBSelector(object, lintError)
		sel := nsLabelSelector{
			namespace: object.Unstructured.GetNamespace(),
			selector:  labelSelector,
		}
		selectors = append(selectors, sel)
	}

	return selectors
}

// ensurePDBIsPresent returns true if there is a PDB controlling pods from the pod contoller
// VPA is assumed to be present, since the PDB check goes after VPA check.
func ensurePDBIsPresent(selectors []nsLabelSelector, podController storage.StoreObject, lintError *errors.Error) {
	podLabels, err := parsePodControllerLabels(podController)
	if err != nil {
		lintError.WithObjectID(podController.Identity()).WithValue(err).
			Add("Cannot parse pod controller")
	}

	podNamespace := podController.Unstructured.GetNamespace()
	podLabelsSet := labels.Set(podLabels)

	for _, sel := range selectors {
		if sel.Matches(podNamespace, podLabelsSet) {
			return
		}
	}

	lintError.WithObjectID(podController.Identity()).WithValue(podLabelsSet).
		Add("No PodDisruptionBudget matches pod labels of controller")
}

// ensurePDBIsNotPresent returns true if there is not a PDB controlling pods from the pod contoller
// VPA is assumed to be present, since the PDB check goes after VPA check.
func ensurePDBIsNotPresent(selectors []nsLabelSelector, podController storage.StoreObject, lintError *errors.Error) {
	podLabels, err := parsePodControllerLabels(podController)
	if err != nil {
		lintError.WithObjectID(podController.Identity()).
			WithValue(err).
			Add("Cannot parse pod controller")
		return
	}

	podNamespace := podController.Unstructured.GetNamespace()
	podLabelsSet := labels.Set(podLabels)

	for _, sel := range selectors {
		if sel.Matches(podNamespace, podLabelsSet) {
			lintError.WithObjectID(podController.Identity()).
				WithValue(podLabelsSet).
				Add("PodDisruptionBudget matches pod labels of controller")
			return
		}
	}
}

func parsePDBSelector(pdbObj storage.StoreObject, lintError *errors.Error) labels.Selector {
	content := pdbObj.Unstructured.UnstructuredContent()
	converter := runtime.DefaultUnstructuredConverter

	pdb := &policyv1.PodDisruptionBudget{}
	err := converter.FromUnstructured(content, pdb)
	if err != nil {
		lintError.WithObjectID(pdbObj.Identity()).WithValue(err).
			Add("Cannot parse PodDisruptionBudget")
		return nil
	}

	sel, err := v1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		lintError.WithObjectID(pdbObj.Identity()).WithValue(err).
			Add("Cannot parse label selector")
		return nil
	}

	if pdb.Annotations["helm.sh/hook"] != "" || pdb.Annotations["helm.sh/hook-delete-policy"] != "" {
		lintError.WithObjectID(pdbObj.Identity()).WithValue(err).
			Add("PDB must have no helm hook annotations")
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
