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
	v2 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

type ContainerVolumeMounts struct {
	ContainerName string
	VolumeMounts  []v2.VolumeMount
}

// controllerMustHavePDB adds linting errors if there are pods from controllers which are not covered (except DaemonSets)
// by a PodDisruptionBudget
func ShowVolumes(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
	v, err := parsePodControllerVolumeMounts(object)
	if v == nil {
		return
	}

	if err != nil {
		errorList.WithObjectID(object.Identity()).
			WithFilePath(object.ShortPath()).
			Error("Error in getting volumes list")
		return
	}

	for _, container := range v {
		fmt.Printf("%s:\n", container.ContainerName)
		for _, vm := range container.VolumeMounts {
			fmt.Printf("  - %s\n", vm.MountPath)
		}
	}
}

func parsePodControllerVolumeMounts(object storage.StoreObject) ([]ContainerVolumeMounts, error) {
	content := object.Unstructured.UnstructuredContent()
	converter := runtime.DefaultUnstructuredConverter
	kind := object.Unstructured.GetKind()

	var containerVolumes []ContainerVolumeMounts

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(content, deployment)
		if err != nil {
			return nil, err
		}

		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

		for _, container := range deployment.Spec.Template.Spec.InitContainers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

	case "DaemonSet":
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(content, daemonSet)
		if err != nil {
			return nil, err
		}

		for _, container := range daemonSet.Spec.Template.Spec.Containers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

		for _, container := range daemonSet.Spec.Template.Spec.InitContainers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

	case "StatefulSet":
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(content, statefulSet)
		if err != nil {
			return nil, err
		}

		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

		for _, container := range statefulSet.Spec.Template.Spec.InitContainers {
			if container.VolumeMounts != nil {
				containerVolumes = append(containerVolumes, ContainerVolumeMounts{ContainerName: container.Name, VolumeMounts: container.VolumeMounts})
			}
		}

	default:
		return nil, fmt.Errorf("object of kind %s is not a pod controller", kind)
	}
	return containerVolumes, nil
}
