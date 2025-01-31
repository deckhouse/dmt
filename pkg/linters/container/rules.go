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

func (o *Container) applyContainerRules(object storage.StoreObject) *errors.LintRuleErrorsList {
	containers, err := object.GetAllContainers()
	if err != nil || len(containers) == 0 {
		return nil
	}

	rules := []func(storage.StoreObject, []v1.Container) *errors.LintRuleErrorsList{
		o.containerNameDuplicates,
		o.containerEnvVariablesDuplicates,
		o.containerImageDigestCheck,
		o.containersImagePullPolicy,
		o.containerStorageEphemeral,
		o.containerSecurityContext,
		o.containerPorts,
	}

	for _, rule := range rules {
		o.result.Merge(rule(object, containers))
	}

	return o.result
}

func (o *Container) containersImagePullPolicy(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	if object.Unstructured.GetNamespace() == "d8-system" &&
		object.Unstructured.GetKind() == "Deployment" &&
		object.Unstructured.GetName() == "deckhouse" {
		return o.checkImagePullPolicyAlways(object, containers)
	}
	return o.containerImagePullPolicyIfNotPresent(object, containers)
}

func (o *Container) checkImagePullPolicyAlways(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	c := containers[0]
	if c.ImagePullPolicy != v1.PullAlways {
		return o.result.WithObjectID(object.Identity()+"; container = "+c.Name).
			AddValue(
				c.ImagePullPolicy,
				`Container imagePullPolicy should be unspecified or "Always"`,
			)
	}
	return nil
}

func (o *Container) containerNameDuplicates(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	return checkForDuplicates(object, containers, func(c v1.Container) string { return c.Name }, "Duplicate container name", o.result)
}

func (o *Container) containerEnvVariablesDuplicates(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			o.result.WithWarning(true)
		}
		if err := checkForDuplicates(object, c.Env, func(e v1.EnvVar) string { return e.Name }, "Container has two env variables with same name", o.result); err != nil {
			return err
		}
	}
	return nil
}

func checkForDuplicates[T any](
	object storage.StoreObject,
	items []T,
	keyFunc func(T) string,
	errMsg string,
	result *errors.LintRuleErrorsList,
) *errors.LintRuleErrorsList {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := keyFunc(item)
		if _, ok := seen[key]; ok {
			return result.WithObjectID(object.Identity()).
				Add("%s", errMsg)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (o *Container) shouldSkipModuleContainer(md, container string) bool {
	for _, line := range o.cfg.SkipContainers {
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

func (o *Container) containerImageDigestCheck(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		if len(match) == 0 {
			return o.result.
				WithObjectID(object.Identity() + "; container = " + c.Name).Add("Cannot parse repository from image")
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			return o.result.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("Cannot parse repository from image: %s", c.Image)
		}

		if repo.Name() != defaultRegistry {
			return o.result.
				WithObjectID(object.Identity()+"; container = "+c.Name).
				Add("All images must be deployed from the same default registry: %s current: %s",
					defaultRegistry,
					repo.RepositoryStr())
		}
	}
	return nil
}

func (o *Container) containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return o.result.
			WithObjectID(object.Identity()+"; container = "+c.Name).
			WithWarning(o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name)).
			AddValue(c.ImagePullPolicy, `Container imagePullPolicy should be unspecified or "IfNotPresent"`)
	}
	return nil
}

func (o *Container) containerStorageEphemeral(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			return o.result.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				WithWarning(o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name)).
				Add("Ephemeral storage for container is not defined in Resources.Requests")
		}
	}
	return nil
}

func (o *Container) containerSecurityContext(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		if c.SecurityContext == nil {
			return o.result.
				WithObjectID(object.Identity() + "; container = " + c.Name).
				WithWarning(o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name)).
				Add("Container SecurityContext is not defined")
		}
	}
	return nil
}

func (o *Container) containerPorts(object storage.StoreObject, containers []v1.Container) *errors.LintRuleErrorsList {
	for i := range containers {
		c := &containers[i]
		for _, p := range c.Ports {
			const t = 1024
			if p.ContainerPort <= t {
				return o.result.
					WithObjectID(object.Identity()+"; container = "+c.Name).
					WithWarning(o.shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name)).
					AddValue(p.ContainerPort, "Container uses port <= 1024")
			}
		}
	}
	return nil
}
