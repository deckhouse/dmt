package container

import (
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func applyContainerRules(object storage.StoreObject, lintError *errors.Error) {
	containers, err := object.GetAllContainers()
	if err != nil || len(containers) == 0 {
		return
	}

	rules := []func(storage.StoreObject, []v1.Container, *errors.Error){
		containerNameDuplicates,
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		containerStorageEphemeral,
		containerSecurityContext,
		containerPorts,
	}

	for _, rule := range rules {
		rule(object, containers, lintError)
	}
}

func containersImagePullPolicy(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	if object.Unstructured.GetNamespace() == "d8-system" && object.Unstructured.GetKind() == "Deployment" && object.Unstructured.GetName() == "deckhouse" {
		checkImagePullPolicyAlways(object, containers, lintError)
		return
	}
	containerImagePullPolicyIfNotPresent(object, containers, lintError)
}

func checkImagePullPolicyAlways(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	c := containers[0]
	if c.ImagePullPolicy != v1.PullAlways {
		lintError.WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "Always"`)
	}
}

func containerNameDuplicates(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	checkForDuplicates(object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name", lintError)
}

func containerEnvVariablesDuplicates(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		checkForDuplicates(object, c.Env, func(e v1.EnvVar) string { return e.Name }, "Container has two env variables with same name", lintError)
	}
}

func checkForDuplicates[T any](object storage.StoreObject, items []T, keyFunc func(T) string, errMsg string, lintError *errors.Error) {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			lintError.WithObjectID(object.Identity()).
				Add("%s", errMsg)
		}
		seen[key] = struct{}{}
	}
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

func containerImageDigestCheck(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			lintError.
				WithObjectID(object.Identity() + "; container = " + c.Name).Add("Cannot parse repository from image")
			continue
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			lintError.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("Cannot parse repository from image: %s", c.Image)
			continue
		}

		if repo.Name() != defaultRegistry {
			lintError.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("All images must be deployed from the same default registry: %s current: %s",
					defaultRegistry,
					repo.RepositoryStr())
		}
	}
}

func containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		lintError.
			WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
}

func containerStorageEphemeral(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			lintError.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				Add("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
}

func containerSecurityContext(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		if c.SecurityContext == nil {
			lintError.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				Add("Container SecurityContext is not defined")
		}
	}
}

func containerPorts(object storage.StoreObject, containers []v1.Container, lintError *errors.Error) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				lintError.
					WithObjectID(object.Identity() + "; container = " + c.Name).
					WithValue(p.ContainerPort).
					Add("Container uses port <= 1024")
			}
		}
	}
}
