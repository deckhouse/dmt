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

func applyContainerRules(m *module.Module, object storage.StoreObject) (result errors.LintRuleErrorsList) {
	containers, err := object.GetContainers()
	if err != nil {
		return
	}
	initContainers, err := object.GetInitContainers()
	if err != nil {
		return
	}
	containers = append(initContainers, containers...)
	if len(containers) == 0 {
		return
	}

	result = errors.LintRuleErrorsList{}

	result.Add(containerNameDuplicates(m.GetName(), object, containers))
	result.Add(containerEnvVariablesDuplicates(m.GetName(), object, containers))
	result.Add(containerImageDigestCheck(m.GetName(), object, containers))
	result.Add(containersImagePullPolicy(m.GetName(), object, containers))

	result.Add(containerStorageEphemeral(m.GetName(), object, containers))
	result.Add(containerSecurityContext(m.GetName(), object, containers))
	result.Add(containerPorts(m.GetName(), object, containers))

	return result
}

func containersImagePullPolicy(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	if len(containers) == 0 {
		return nil
	}
	ob := object.Unstructured
	if ob.GetNamespace() == "d8-system" && ob.GetKind() == "Deployment" && ob.GetName() == "deckhouse" {
		c := containers[0]
		if c.ImagePullPolicy != "Always" {
			// image pull policy must be Always,
			// because changing d8-system/deckhouse-registry triggers restart deckhouse deployment
			// d8-system/deckhouse-registry can contain invalid registry creds
			// and restarting deckhouse with invalid creads will break all static pods on masters
			// and bashible
			return errors.NewLintRuleError(
				ID,
				object.Identity()+"; container = "+c.Name,
				module,
				c.ImagePullPolicy,
				`Container imagePullPolicy should be unspecified or "Always"`,
			)
		}

		return nil
	}

	return containerImagePullPolicyIfNotPresent(module, object, containers)
}

func containerNameDuplicates(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	names := make(map[string]struct{})
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		if _, ok := names[containers[i].Name]; ok {
			return errors.NewLintRuleError(ID, object.Identity(), module, nil, "Duplicate container name")
		}
		names[containers[i].Name] = struct{}{}
	}
	return nil
}

func containerEnvVariablesDuplicates(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		envVariables := make(map[string]struct{})
		for _, variable := range containers[i].Env {
			if _, ok := envVariables[variable.Name]; ok {
				return errors.NewLintRuleError(
					ID,
					object.Identity()+"; container = "+containers[i].Name,
					module,
					variable.Name,
					"Container has two env variables with same name",
				)
			}
			envVariables[variable.Name] = struct{}{}
		}
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

func containerImageDigestCheck(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(containers[i].Image)
		if len(match) == 0 {
			return errors.NewLintRuleError(ID,
				object.Identity()+"; container = "+containers[i].Name,
				module,
				nil,
				"Cannot parse repository from image",
			)
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			return errors.NewLintRuleError(ID,
				object.Identity()+"; container = "+containers[i].Name,
				module,
				nil,
				"Cannot parse repository from image: %s", containers[i].Image,
			)
		}

		if repo.Name() != defaultRegistry {
			return errors.NewLintRuleError(ID,
				object.Identity()+"; container = "+containers[i].Name,
				module,
				nil,
				"All images must be deployed from the same default registry: %s current: %s",
				defaultRegistry,
				repo.RepositoryStr(),
			)
		}
	}
	return nil
}

func containerImagePullPolicyIfNotPresent(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		if containers[i].ImagePullPolicy == "" || containers[i].ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return errors.NewLintRuleError(
			ID,
			object.Identity()+"; container = "+containers[i].Name,
			module,
			containers[i].ImagePullPolicy,
			`Container imagePullPolicy should be unspecified or "IfNotPresent"`,
		)
	}
	return nil
}

func containerStorageEphemeral(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		if containers[i].Resources.Requests.StorageEphemeral() == nil ||
			containers[i].Resources.Requests.StorageEphemeral().Value() == 0 {
			return errors.NewLintRuleError(
				ID,
				object.Identity()+"; container = "+containers[i].Name,
				module,
				nil,
				"Ephemeral storage for container is not defined in Resources.Requests",
			)
		}
	}
	return nil
}

func containerSecurityContext(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		if containers[i].SecurityContext == nil {
			return errors.NewLintRuleError(
				ID,
				object.Identity()+"; container = "+containers[i].Name,
				module,
				nil,
				"Container SecurityContext is not defined",
			)
		}
	}
	return nil
}

func containerPorts(module string, object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}
		for _, p := range containers[i].Ports {
			const t = 1024
			if p.ContainerPort <= t {
				return errors.NewLintRuleError(
					ID,
					object.Identity()+"; container = "+containers[i].Name,
					module,
					p.ContainerPort,
					"Container uses port <= 1024",
				)
			}
		}
	}
	return nil
}
