package probes

import (
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/module"
	"github.com/deckhouse/d8-lint/pkg/storage"
)

// Probes linter
type Probes struct {
	name, desc string
	cfg        *config.ProbesSettings
}

func New(cfg *config.ProbesSettings) *Probes {
	return &Probes{
		name: "probes",
		desc: "Probes will check all containers for correct liveness and readiness probes",
		cfg:  cfg,
	}
}

func (o *Probes) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	var result errors.LintRuleErrorsList
	var err error

	{
		m.Chart, err = loader.Load(m.GetPath())
		if err != nil {
			return result, err
		}
	}

	var values []chartutil.Values
	{
		values, err = ComposeValuesFromSchemas(m)
		if err != nil {
			return result, fmt.Errorf("saving values from openapi: %v", err)
		}
	}

	for _, valuesData := range values {
		objectStore := storage.NewUnstructuredObjectStore()
		err = RunRender(m, valuesData, objectStore)

		for _, object := range objectStore.Storage {
			containers, err := object.GetContainers()
			if err != nil || containers == nil {
				continue
			}

			result = o.containerProbes(m.GetName(), object, containers, result)
		}

	}

	return result, nil
}

func (o *Probes) Name() string {
	return o.name
}

func (o *Probes) Desc() string {
	return o.desc
}

func (o *Probes) containerProbes(
	moduleName string,
	object storage.StoreObject,
	containers []v1.Container,
	errorList errors.LintRuleErrorsList,
) errors.LintRuleErrorsList {
	for _, container := range containers {
		if o.skipCheckProbeHandler(object.Unstructured.GetNamespace(), container.Name) {
			continue
		}

		var errStrings []string
		// check livenessProbe exist and correct
		livenessProbe := container.LivenessProbe
		if livenessProbe == nil || probeHandlerIsNotValid(livenessProbe.ProbeHandler) {
			errStrings = append(errStrings, "LivenessProbe")
		}

		// check readinessProbe exist and correct
		readinessProbe := container.ReadinessProbe
		if readinessProbe == nil || probeHandlerIsNotValid(readinessProbe.ProbeHandler) {
			errStrings = append(errStrings, "ReadinessProbe")
		}

		if len(errStrings) > 0 {
			errorList.Add(errors.NewLintRuleError(
				"probes",
				"module = "+moduleName+" ; "+object.Identity()+" ; container = "+container.Name,
				nil,
				"Container does not use correct "+strings.Join(errStrings, " and "),
			))
		}
	}

	return errorList
}

func probeHandlerIsNotValid(probe v1.ProbeHandler) bool {
	var count int8
	if probe.Exec != nil {
		count++
	}
	if probe.GRPC != nil {
		count++
	}
	if probe.HTTPGet != nil {
		count++
	}
	if probe.TCPSocket != nil {
		count++
	}
	if count != 1 {
		return true
	}

	return false
}

func (o *Probes) skipCheckProbeHandler(namespace, container string) bool {
	containers, ok := o.cfg.ProbesExcludes[namespace]
	if ok {
		_, ok = containers[container]
		return ok
	}

	return false
}
