package probes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sourcegraph/conc/pool"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/k8s"
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

	values, err := k8s.ComposeValuesFromSchemas(m)
	if err != nil {
		return result, fmt.Errorf("saving values from openapi: %w", err)
	}

	var ch = make(chan errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithErrors()
		g.Go(func() error {
			objectStore := storage.NewUnstructuredObjectStore()
			err = k8s.RunRender(m, values, objectStore)
			if err != nil {
				return err
			}

			for _, object := range objectStore.Storage {
				containers, er := object.GetContainers()
				if er != nil || containers == nil {
					continue
				}
				ch <- o.containerProbes(m.GetName(), object, containers)
			}

			return nil
		})
		err = g.Wait()
		close(ch)
	}()

	for er := range ch {
		result.Merge(er)
	}

	return result, err
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
) errors.LintRuleErrorsList {
	var errorList errors.LintRuleErrorsList
	for i := range containers {
		container := containers[i]
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
				moduleName,
				strings.Join(errStrings, " and "),
				"Container does not use correct probes",
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
		return slices.Contains(containers, container)
	}

	return false
}
