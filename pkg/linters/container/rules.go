package container

import (
	"regexp"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/config"
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
		return errors.NewLinterRuleList(ID, md).WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "Always"`)
	}
	return nil
}

func containerNameDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	return checkForDuplicates(md, object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name")
}

func containerEnvVariablesDuplicates(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if err := checkForDuplicates(md, object, c.Env, func(e v1.EnvVar) string { return e.Name }, "Container has two env variables with same name"); err != nil {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
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
			config.GlobalExcludes.Container.SkipContainers = slices.DeleteFunc(
				config.GlobalExcludes.Container.SkipContainers,
				func(cmp string) bool {
					return cmp == line
				},
			)

			return true
		}
	}

	return false
}

func containerImageDigestCheck(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity() + "; container = " + c.Name).Add("Cannot parse repository from image")
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
			return errors.NewLinterRuleList(ID, md).
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("Cannot parse repository from image: %s", c.Image)
		}

		if repo.Name() != defaultRegistry {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
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
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		if shouldSkipModuleContainer(md, c.Name) {
			continue
		}
		return errors.NewLinterRuleList(ID, md).
			WithObjectID(object.Identity() + "; container = " + c.Name).
			WithValue(c.ImagePullPolicy).
			Add(`Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
	return nil
}

func containerStorageEphemeral(md string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
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
		if c.SecurityContext == nil {
			if shouldSkipModuleContainer(md, c.Name) {
				continue
			}
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
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				if shouldSkipModuleContainer(md, c.Name) {
					continue
				}
				return errors.NewLinterRuleList(ID, md).
					WithObjectID(object.Identity() + "; container = " + c.Name).
					WithValue(p.ContainerPort).
					Add("Container uses port <= 1024")
			}
		}
	}
	return nil
}
