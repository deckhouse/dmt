package probes

import (
	"slices"
	"strings"

	"github.com/sourcegraph/conc/pool"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Probes linter
type Probes struct {
	name, desc string
	cfg        *config.ProbesSettings
}

var Cfg *config.ProbesSettings

func New(cfg *config.ProbesSettings) *Probes {
	Cfg = cfg
	return &Probes{
		name: "probes",
		desc: "Probes will check all containers for correct liveness and readiness probes",
		cfg:  cfg,
	}
}

func (*Probes) Run(m *module.Module) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("probes", m.GetName())
	var err error
	var ch = make(chan *errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithErrors()
		g.Go(func() error {
			for _, object := range m.GetStorage() {
				containers, er := object.GetContainers()
				if er != nil || containers == nil {
					continue
				}
				ch <- containerProbes(m.GetName(), object, containers)
			}

			return nil
		})
		err = g.Wait()
		close(ch)
	}()

	for er := range ch {
		result.Merge(er)
	}

	if err != nil {
		result.WithObjectID(
			"module = "+m.GetName()).AddValue(
			err.Error(),
			"Error in probes linter",
		)
	}

	return result
}

func (o *Probes) Name() string {
	return o.name
}

func (o *Probes) Desc() string {
	return o.desc
}

func containerProbes(
	moduleName string,
	object storage.StoreObject,
	containers []v1.Container,
) *errors.LintRuleErrorsList {
	result := errors.NewLinterRuleList("probes", moduleName)
	for i := range containers {
		container := containers[i]
		if skipCheckProbeHandler(object.Unstructured.GetNamespace(), container.Name) {
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
			result.WithObjectID("module = "+moduleName+" ; "+object.Identity()+" ; container = "+container.Name).
				AddValue(
					strings.Join(errStrings, " and "),
					"Container does not use correct probes",
				)
		}
	}

	return result
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

func skipCheckProbeHandler(namespace, container string) bool {
	containers, ok := Cfg.ProbesExcludes[namespace]
	if ok {
		return slices.Contains(containers, container)
	}

	return false
}
