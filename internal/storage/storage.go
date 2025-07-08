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

package storage

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	deploymentString  = "Deployment"
	daemonSetString   = "DaemonSet"
	statefulSetString = "StatefulSet"
	podString         = "Pod"
	jobString         = "Job"
	cronJobString     = "CronJob"
	replicaSetString  = "ReplicaSet"
)

type ResourceIndex struct {
	Kind      string
	Name      string
	Namespace string
}

func (g *ResourceIndex) AsString() string {
	if g.Namespace == "" {
		return g.Kind + "/" + g.Name
	}

	return g.Namespace + "/" + g.Kind + "/" + g.Name
}

type StoreObject struct {
	Path         string
	Hash         string
	Unstructured unstructured.Unstructured
}

func GetResourceIndex(object StoreObject) ResourceIndex {
	return ResourceIndex{
		Kind:      object.Unstructured.GetKind(),
		Name:      object.Unstructured.GetName(),
		Namespace: object.Unstructured.GetNamespace(),
	}
}

func (s *StoreObject) GetInitContainers() ([]v1.Container, error) {
	var containers []v1.Container
	converter := runtime.DefaultUnstructuredConverter

	switch s.Unstructured.GetKind() {
	case deploymentString:
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Deployment failed: %w", err)
		}

		containers = deployment.Spec.Template.Spec.InitContainers
	case daemonSetString:
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), daemonSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to DaemonSet failed: %w", err)
		}

		containers = daemonSet.Spec.Template.Spec.InitContainers
	case statefulSetString:
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), statefulSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to StatefulSet failed: %w", err)
		}

		containers = statefulSet.Spec.Template.Spec.InitContainers
	case podString:
		pod := new(v1.Pod)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), pod)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Pod failed: %w", err)
		}

		containers = pod.Spec.InitContainers
	case jobString:
		job := new(batchv1.Job)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), job)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Job failed: %w", err)
		}

		containers = job.Spec.Template.Spec.InitContainers
	case cronJobString:
		cronJob := new(batchv1.CronJob)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), cronJob)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to CronJob failed: %w", err)
		}

		containers = cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers
	}
	return containers, nil
}

func (s *StoreObject) GetContainers() ([]v1.Container, error) {
	var containers []v1.Container
	converter := runtime.DefaultUnstructuredConverter

	switch s.Unstructured.GetKind() {
	case deploymentString:
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Deployment failed: %w", err)
		}

		containers = deployment.Spec.Template.Spec.Containers
	case daemonSetString:
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), daemonSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to DaemonSet failed: %w", err)
		}

		containers = daemonSet.Spec.Template.Spec.Containers
	case statefulSetString:
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), statefulSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to StatefulSet failed: %w", err)
		}

		containers = statefulSet.Spec.Template.Spec.Containers
	case podString:
		pod := new(v1.Pod)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), pod)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Pod failed: %w", err)
		}

		containers = pod.Spec.Containers
	case jobString:
		job := new(batchv1.Job)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), job)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Job failed: %w", err)
		}

		containers = job.Spec.Template.Spec.Containers
	case cronJobString:
		cronJob := new(batchv1.CronJob)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), cronJob)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to CronJob failed: %w", err)
		}

		containers = cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
	case replicaSetString:
		replicaSet := new(appsv1.ReplicaSet)
		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), replicaSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to ReplicaSet failed: %w", err)
		}

		containers = replicaSet.Spec.Template.Spec.Containers
	}
	return containers, nil
}

func (s *StoreObject) GetAllContainers() ([]v1.Container, error) {
	containers, err := s.GetContainers()
	if err != nil {
		return nil, err
	}

	initContainers, err := s.GetInitContainers()
	if err != nil {
		return nil, err
	}

	return append(containers, initContainers...), nil
}

func (s *StoreObject) GetPodSecurityContext() (*v1.PodSecurityContext, error) {
	converter := runtime.DefaultUnstructuredConverter

	switch s.Unstructured.GetKind() {
	case deploymentString:
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to Deployment failed: %w", err)
		}

		return deployment.Spec.Template.Spec.SecurityContext, nil
	case daemonSetString:
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), daemonSet)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to DaemonSet failed: %w", err)
		}

		return daemonSet.Spec.Template.Spec.SecurityContext, nil
	case statefulSetString:
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), statefulSet)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to StatefulSet failed: %w", err)
		}

		return statefulSet.Spec.Template.Spec.SecurityContext, nil
	case podString:
		pod := new(v1.Pod)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), pod)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to Pod failed: %w", err)
		}

		return pod.Spec.SecurityContext, nil
	case jobString:
		job := new(batchv1.Job)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), job)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to Job failed: %w", err)
		}

		return job.Spec.Template.Spec.SecurityContext, nil
	case cronJobString:
		cronJob := new(batchv1.CronJob)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), cronJob)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to CronJob failed: %w", err)
		}

		return cronJob.Spec.JobTemplate.Spec.Template.Spec.SecurityContext, nil
	case replicaSetString:
		replicaSet := new(appsv1.ReplicaSet)
		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), replicaSet)
		if err != nil {
			return nil, fmt.Errorf("convert Unstructured to ReplicaSet failed: %w", err)
		}

		return replicaSet.Spec.Template.Spec.SecurityContext, nil
	}
	return nil, nil
}

func (s *StoreObject) IsHostNetwork() (bool, error) {
	converter := runtime.DefaultUnstructuredConverter

	switch s.Unstructured.GetKind() {
	case deploymentString:
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to Deployment failed: %w", err)
		}

		return deployment.Spec.Template.Spec.HostNetwork, nil
	case daemonSetString:
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), daemonSet)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to DaemonSet failed: %w", err)
		}

		return daemonSet.Spec.Template.Spec.HostNetwork, nil
	case statefulSetString:
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), statefulSet)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to StatefulSet failed: %w", err)
		}

		return statefulSet.Spec.Template.Spec.HostNetwork, nil
	case podString:
		pod := new(v1.Pod)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), pod)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to Pod failed: %w", err)
		}

		return pod.Spec.HostNetwork, nil
	case jobString:
		job := new(batchv1.Job)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), job)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to Job failed: %w", err)
		}

		return job.Spec.Template.Spec.HostNetwork, nil
	case cronJobString:
		cronJob := new(batchv1.CronJob)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), cronJob)
		if err != nil {
			return false, fmt.Errorf("convert Unstructured to CronJob failed: %w", err)
		}

		return cronJob.Spec.JobTemplate.Spec.Template.Spec.HostNetwork, nil
	}
	return false, nil
}

func (s *StoreObject) ShortPath() string {
	elements := strings.Split(s.Path, string(os.PathSeparator))
	if len(elements) == 0 {
		return ""
	}
	path := elements[1:]
	return strings.Join(path, string(os.PathSeparator))
}

func (s *StoreObject) Identity() string {
	kind := s.Unstructured.GetKind()
	name := s.Unstructured.GetName()
	namespace := s.Unstructured.GetNamespace()

	if namespace == "" {
		return fmt.Sprintf("kind = %s ; name = %s", kind, name)
	}
	return fmt.Sprintf("kind = %s ; name = %s ; namespace = %s", kind, name, namespace)
}

type UnstructuredObjectStore struct {
	Storage map[ResourceIndex]StoreObject
}

func NewUnstructuredObjectStore() *UnstructuredObjectStore {
	return &UnstructuredObjectStore{Storage: make(map[ResourceIndex]StoreObject)}
}

// isDuplicateAllowed checks if a duplicate object is allowed based on specific conditions
func isDuplicateAllowed(index ResourceIndex) bool {
	indexStr := index.AsString()
	// for cert-manager migration we have duplicated resources for legacy version
	// it's ok for cluster but is not expected by tests. Remove it after legacy version will be removed
	return strings.Contains(indexStr, "ClusterIssuer") || strings.HasPrefix(indexStr, "d8-cert-manager")
}

func (s *UnstructuredObjectStore) Put(path string, object map[string]any, raw []byte) error {
	var u unstructured.Unstructured
	u.SetUnstructuredContent(object)

	storeObject := StoreObject{Path: path, Unstructured: u, Hash: NewSHA256(raw)}

	var err error
	index := GetResourceIndex(storeObject)
	if _, ok := s.Storage[index]; ok {
		if isDuplicateAllowed(index) {
			return nil
		}
		err = fmt.Errorf("object %q already exists", index.AsString())
	}

	s.Storage[index] = storeObject
	return err
}

func (s *UnstructuredObjectStore) Get(key ResourceIndex) StoreObject {
	return s.Storage[key]
}

func (s *UnstructuredObjectStore) Exists(key ResourceIndex) bool {
	_, ok := s.Storage[key]
	return ok
}

func (s *UnstructuredObjectStore) Close() {
	s.Storage = make(map[ResourceIndex]StoreObject)
}

func NewSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}
