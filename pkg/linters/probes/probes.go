package probes

import (
	"slices"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/dmt/internal/logger"
	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/internal/storage"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
)

// Probes linter
type Probes struct {
	name string
	cfg  *config.ProbesSettings
}

func Run(m *module.Module) {
	p := &Probes{
		name: "probes",
		cfg:  &config.Cfg.LintersSettings.Probes,
	}

	logger.DebugF("Running linter `%s` on module `%s`", p.name, m.GetName())

	for _, object := range m.GetStorage() {
		containers, er := object.GetContainers()
		if er != nil || containers == nil {
			continue
		}
		p.containerProbes(m.GetName(), object, containers)
	}
}

func (p *Probes) containerProbes(
	moduleName string,
	object storage.StoreObject,
	containers []v1.Container,
) {
	lintError := errors.NewError("probes", moduleName)
	for i := range containers {
		container := containers[i]
		if p.skipCheckProbeHandler(object.Unstructured.GetNamespace(), container.Name) {
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
			lintError.WithObjectID("module = " + moduleName + " ; " + object.Identity() + " ; container = " + container.Name).
				WithValue(strings.Join(errStrings, " and ")).
				Add("Container does not use correct probes")
		}
	}
}

func (p *Probes) skipCheckProbeHandler(namespace, container string) bool {
	containers, ok := p.cfg.ProbesExcludes[namespace]
	if ok {
		return slices.Contains(containers, container)
	}

	return false
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
