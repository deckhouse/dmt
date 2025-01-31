package container

import (
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func applyContainerRules(object storage.StoreObject, result *errors.LintRuleErrorsList, cfg *config.ContainerSettings) {
	containers, err := object.GetAllContainers()
	if err != nil || len(containers) == 0 {
		return // skip
	}

	rules := []func(storage.StoreObject, []v1.Container, *config.ContainerSettings, *errors.LintRuleErrorsList){
		containerNameDuplicates,
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		containerStorageEphemeral,
		containerSecurityContext,
		containerPorts,
	}

	for _, rule := range rules {
		rule(object, containers, cfg, result)
	}
}

func containersImagePullPolicy(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	if object.Unstructured.GetNamespace() == "d8-system" &&
		object.Unstructured.GetKind() == "Deployment" &&
		object.Unstructured.GetName() == "deckhouse" {
		checkImagePullPolicyAlways(object, containers, cfg, result)
		return
	}
	containerImagePullPolicyIfNotPresent(object, containers, cfg, result)
}

func checkImagePullPolicyAlways(object storage.StoreObject, containers []v1.Container, _ *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	c := containers[0]
	if c.ImagePullPolicy != v1.PullAlways {
		result.WithObjectID(object.Identity()+"; container = "+c.Name).
			AddValue(
				c.ImagePullPolicy,
				`Container imagePullPolicy should be unspecified or "Always"`,
			)
	}
}

func containerNameDuplicates(object storage.StoreObject, containers []v1.Container, _ *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	checkForDuplicates(object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name", result)
}

func containerEnvVariablesDuplicates(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg) {
			result.WithWarning(true)
		}
		checkForDuplicates(object, c.Env, func(e v1.EnvVar) string { return e.Name }, "Container has two env variables with same name", result)
	}
}

func checkForDuplicates[T any](
	object storage.StoreObject,
	items []T,
	keyFunc func(T) string,
	errMsg string,
	result *errors.LintRuleErrorsList,
) {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			result.WithObjectID(object.Identity()).
				Add("%s", errMsg)
		}
		seen[key] = struct{}{}
	}
}

func shouldSkipModuleContainer(md, container string, cfg *config.ContainerSettings) bool {
	for _, line := range cfg.SkipContainers {
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

func containerImageDigestCheck(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			result.
				WithObjectID(object.Identity() + "; container = " + c.Name).Add("Cannot parse repository from image")
			return
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			result.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("Cannot parse repository from image: %s", c.Image)
			return
		}

		if repo.Name() != defaultRegistry {
			result.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("All images must be deployed from the same default registry: %s current: %s",
					defaultRegistry,
					repo.RepositoryStr())
			return
		}
	}
}

func containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		result.
			WithObjectID(object.Identity()+"; container = "+c.Name).
			WithWarning(shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg)).
			AddValue(c.ImagePullPolicy, `Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
}

func containerStorageEphemeral(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			result.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				WithWarning(shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg)).
				Add("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
}

func containerSecurityContext(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		if c.SecurityContext == nil {
			result.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				WithWarning(shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg)).
				Add("Container SecurityContext is not defined")
		}
	}
}

func containerPorts(object storage.StoreObject, containers []v1.Container, cfg *config.ContainerSettings, result *errors.LintRuleErrorsList) {
	for i := range containers {
		c := &containers[i]
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				result.
					WithObjectID(object.Identity()+"; container = "+c.Name).
					WithWarning(shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name, cfg)).
					AddValue(p.ContainerPort, "Container uses port <= 1024")
			}
		}
	}
}
