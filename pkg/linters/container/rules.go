package container

import (
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func applyContainerRules(m *module.Module, object storage.StoreObject) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList(ID, m.GetName())
	containers, err := object.GetAllContainers()
	if err != nil || len(containers) == 0 {
		return result
	}

	rules := []func(string, storage.StoreObject, []v1.Container) *errors.LintRuleErrorsList{
		containerNameDuplicates,
		containerEnvVariablesDuplicates,
		containerImageDigestCheck,
		containersImagePullPolicy,
		containerStorageEphemeral,
		containerSecurityContext,
		containerPorts,
	}

	for _, rule := range rules {
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
		return errors.NewLinterRuleList(ID, md).WithObjectID(object.Identity()+"; container = "+c.Name).
			AddValue(
				c.ImagePullPolicy,
				`Container imagePullPolicy should be unspecified or "Always"`,
			)
	}
	return nil
}

func containerNameDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	return checkForDuplicates(md, object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name")
}

func containerEnvVariablesDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
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
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
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
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return errors.NewLinterRuleList(ID, md).
			WithObjectID(object.Identity()+"; container = "+c.Name).
			AddValue(c.ImagePullPolicy, `Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
	return nil
}

func containerStorageEphemeral(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
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
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
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
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				return errors.NewLinterRuleList(ID, md).
					WithObjectID(object.Identity()+"; container = "+c.Name).
					AddValue(p.ContainerPort, "Container uses port <= 1024")
			}
		}
	}
	return nil
}
