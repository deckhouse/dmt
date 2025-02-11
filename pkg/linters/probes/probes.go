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

const (
	ID = "probes"
)

// Probes linter
type Probes struct {
	name, desc string
	cfg        *config.ProbesSettings
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Probes {
	return &Probes{
		name:      ID,
		desc:      "Probes will check all containers for correct liveness and readiness probes",
		cfg:       &cfg.LintersSettings.Probes,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Probes.Impact),
	}
}

func (l *Probes) Run(m *module.Module) {
	errorList := l.ErrorList.WithModule(m.GetName())

	var err error

	go func() {
		var g = pool.New().WithErrors()

		g.Go(func() error {
			for _, object := range m.GetStorage() {
				containers, er := object.GetContainers()
				if er != nil || containers == nil {
					continue
				}

				l.containerProbes(object, containers, errorList)
			}

			return nil
		})

		err = g.Wait()
	}()

	if err != nil {
		l.ErrorList.Errorf("Error in probes linter: %s", err)
	}
}

func (l *Probes) Name() string {
	return l.name
}

func (l *Probes) Desc() string {
	return l.desc
}

func (l *Probes) containerProbes(
	object storage.StoreObject,
	containers []v1.Container,
	errorList *errors.LintRuleErrorsList,
) *errors.LintRuleErrorsList {
	for i := range containers {
		container := containers[i]

		if l.skipCheckProbeHandler(object.Unstructured.GetNamespace(), container.Name) {
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
			errorList.WithObjectID(object.Identity()+" ; container = "+container.Name).
				Errorf("Container does not use correct probes: %s", strings.Join(errStrings, " and "))
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

func (l *Probes) skipCheckProbeHandler(namespace, container string) bool {
	containers, ok := l.cfg.ProbesExcludes[namespace]
	if ok {
		return slices.Contains(containers, container)
	}

	return false
}
