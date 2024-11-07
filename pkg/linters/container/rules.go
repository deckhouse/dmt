package container

import (
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

const defaultRegistry = "registry.example.com/deckhouse"

func (o *Container) applyContainerRules(object storage.StoreObject) (result errors.LintRuleErrorsList) {
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

	result.Add(o.containerNameDuplicates(object, containers))
	result.Add(o.containerEnvVariablesDuplicates(object, containers))
	result.Add(o.containerImageDigestCheck(object, containers))
	result.Add(o.containersImagePullPolicy(object, containers))

	result.Add(o.containerStorageEphemeral(object, containers))
	result.Add(o.containerSecurityContext(object, containers))
	result.Add(o.containerPorts(object, containers))

	return result
}

func (o *Container) containersImagePullPolicy(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
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
				o.Name(),
				object.Identity()+"; container = "+c.Name,
				c.Name,
				c.ImagePullPolicy,
				`Container imagePullPolicy should be unspecified or "Always"`,
			)
		}

		return errors.EmptyRuleError
	}

	return o.containerImagePullPolicyIfNotPresent(object, containers)
}

func (o *Container) containerNameDuplicates(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	names := make(map[string]struct{})
	for i := range containers {
		if _, ok := names[containers[i].Name]; ok {
			return errors.NewLintRuleError(o.Name(), object.Identity(), containers[i].Name, nil, "Duplicate container name")
		}
		names[containers[i].Name] = struct{}{}
	}
	return errors.EmptyRuleError
}

func (o *Container) containerEnvVariablesDuplicates(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		envVariables := make(map[string]struct{})
		for _, variable := range containers[i].Env {
			if _, ok := envVariables[variable.Name]; ok {
				return errors.NewLintRuleError(
					o.Name(),
					object.Identity()+"; container = "+containers[i].Name,
					containers[i].Name,
					variable.Name,
					"Container has two env variables with same name",
				)
			}
			envVariables[variable.Name] = struct{}{}
		}
	}
	return errors.EmptyRuleError
}

func (o *Container) shouldSkipModuleContainer(md, container string) bool {
	// okmeter module uses images from external repo - registry.okmeter.io/agent/okagent:stub
	if md == "okmeter" && container == "okagent" {
		return true
	}
	// control-plane-manager uses `$images` as dict to render static pod manifests,
	// so we cannot use helm lib `helm_lib_module_image` helper because `$images`
	// is also rendered in `dhctl` tool on cluster bootstrap.
	if md == "d8-control-plane-manager" && strings.HasPrefix(container, "image-holder") {
		return true
	}
	return false
}

func (o *Container) containerImageDigestCheck(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if o.shouldSkipModuleContainer(object.Unstructured.GetName(), containers[i].Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)([@:])imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(containers[i].Image)
		if len(match) == 0 {
			return errors.NewLintRuleError(o.Name(),
				object.Identity()+"; container = "+containers[i].Name,
				object.Unstructured.GetName(),
				nil,
				"Cannot parse repository from image",
			)
		}
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			return errors.NewLintRuleError(o.Name(),
				object.Identity()+"; container = "+containers[i].Name,
				object.Unstructured.GetName(),
				nil,
				"Cannot parse repository from image: %s", containers[i].Image,
			)
		}

		if repo.Name() != defaultRegistry {
			return errors.NewLintRuleError(o.Name(),
				object.Identity()+"; container = "+containers[i].Name,
				object.Unstructured.GetName(),
				nil,
				"All images must be deployed from the same default registry: %s current: %s",
				defaultRegistry,
				repo.RepositoryStr(),
			)
		}
	}
	return errors.EmptyRuleError
}

func (o *Container) containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if containers[i].ImagePullPolicy == "" || containers[i].ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return errors.NewLintRuleError(
			o.Name(),
			object.Identity()+"; container = "+containers[i].Name,
			containers[i].Name,
			containers[i].ImagePullPolicy,
			`Container imagePullPolicy should be unspecified or "IfNotPresent"`,
		)
	}
	return errors.EmptyRuleError
}

func (o *Container) containerStorageEphemeral(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if containers[i].Resources.Requests.StorageEphemeral() == nil ||
			containers[i].Resources.Requests.StorageEphemeral().Value() == 0 {
			return errors.NewLintRuleError(
				o.Name(),
				object.Identity()+"; container = "+containers[i].Name,
				containers[i].Name,
				nil,
				"Ephemeral storage for container is not defined in Resources.Requests",
			)
		}
	}
	return errors.EmptyRuleError
}

func (o *Container) containerSecurityContext(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		if containers[i].SecurityContext == nil {
			return errors.NewLintRuleError(
				o.Name(),
				object.Identity()+"; container = "+containers[i].Name,
				containers[i].Name,
				nil,
				"Container SecurityContext is not defined",
			)
		}
	}
	return errors.EmptyRuleError
}

func (o *Container) containerPorts(object storage.StoreObject, containers []v1.Container) *errors.LintRuleError {
	for i := range containers {
		for _, p := range containers[i].Ports {
			const t = 1024
			if p.ContainerPort <= t {
				return errors.NewLintRuleError(
					o.Name(),
					object.Identity()+"; container = "+containers[i].Name,
					containers[i].Name,
					p.ContainerPort,
					"Container uses port <= 1024",
				)
			}
		}
	}
	return errors.EmptyRuleError
}
