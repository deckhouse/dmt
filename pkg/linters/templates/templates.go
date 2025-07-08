/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package templates

import (
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/module"
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/exclusions"
	"github.com/deckhouse/dmt/pkg/linters/templates/rules"
)

const (
	ID = "templates"
)

// Templates linter
type Templates struct {
	name, desc string
	cfg        *config.TemplatesSettings
	ErrorList  *errors.LintRuleErrorsList
	tracker    *exclusions.ExclusionTracker
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates",
		cfg:       &cfg.LintersSettings.Templates,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Templates.Impact),
	}
}

func NewWithTracker(cfg *config.ModuleConfig, tracker *exclusions.ExclusionTracker, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates (with exclusion tracking)",
		cfg:       &cfg.LintersSettings.Templates,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Templates.Impact),
		tracker:   tracker,
	}
}

func (l *Templates) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	if l.tracker != nil {
		l.runWithTracking(m, m.GetName(), errorList)
	} else {
		l.runWithoutTracking(m, errorList)
	}
}

func (l *Templates) runWithoutTracking(m *module.Module, errorList *errors.LintRuleErrorsList) {
	// Apply VPA and PDB rules
	l.applyVPAPDBRules(m, nil, errorList)

	// Apply monitoring rules
	l.applyMonitoringRules(m, nil, errorList)

	// Apply KubeRBACProxy rule
	l.applyKubeRBACProxyRule(m, nil, errorList)

	// Apply service port and prometheus rules
	l.applyServicePortAndPrometheusRules(m, nil, errorList)
}

func (l *Templates) runWithTracking(m *module.Module, moduleName string, errorList *errors.LintRuleErrorsList) {
	// Register rules without exclusions in tracker
	l.tracker.RegisterExclusionsForModule(ID, "prometheus-rules", []string{}, moduleName)

	// Apply VPA and PDB rules with tracking
	l.applyVPAPDBRules(m, &moduleName, errorList)

	// Apply monitoring rules with tracking
	l.applyMonitoringRules(m, &moduleName, errorList)

	// Apply KubeRBACProxy rule with tracking
	l.applyKubeRBACProxyRule(m, &moduleName, errorList)

	// Apply service port and prometheus rules with tracking
	l.applyServicePortAndPrometheusRules(m, &moduleName, errorList)
}

// applyVPAPDBRules applies VPA and PDB rules with optional tracking
func (l *Templates) applyVPAPDBRules(m *module.Module, moduleName *string, errorList *errors.LintRuleErrorsList) {
	if moduleName != nil {
		// With tracking
		trackedVPARule := exclusions.NewTrackedKindRuleForModule(
			l.cfg.ExcludeRules.VPAAbsent.Get(),
			l.tracker,
			ID,
			"vpa",
			*moduleName,
		)
		rules.NewVPARuleTracked(trackedVPARule).ControllerMustHaveVPA(m, errorList)

		trackedPDBRule := exclusions.NewTrackedKindRuleForModule(
			l.cfg.ExcludeRules.PDBAbsent.Get(),
			l.tracker,
			ID,
			"pdb",
			*moduleName,
		)
		rules.NewPDBRuleTracked(trackedPDBRule).ControllerMustHavePDB(m, errorList)
	} else {
		// Without tracking
		rules.NewVPARule(l.cfg.ExcludeRules.VPAAbsent.Get()).ControllerMustHaveVPA(m, errorList)
		rules.NewPDBRule(l.cfg.ExcludeRules.PDBAbsent.Get()).ControllerMustHavePDB(m, errorList)
	}
}

// applyMonitoringRules applies monitoring rules with optional tracking
func (l *Templates) applyMonitoringRules(m *module.Module, moduleName *string, errorList *errors.LintRuleErrorsList) {
	prometheusRule := rules.NewPrometheusRule()

	if err := dirExists(m.GetPath(), "monitoring"); err == nil {
		if moduleName != nil {
			// Grafana dashboards with tracking
			trackedGrafanaRule := exclusions.NewTrackedBoolRuleForModule(
				l.cfg.GrafanaDashboards.Disable,
				l.tracker,
				ID,
				"grafana-dashboards",
				*moduleName,
			)
			grafanaRule := rules.NewGrafanaRuleTracked(trackedGrafanaRule)
			grafanaRule.ValidateGrafanaDashboards(m, errorList)
		} else {
			// Without tracking
			grafanaRule := rules.NewGrafanaRule(l.cfg)
			grafanaRule.ValidateGrafanaDashboards(m, errorList)
		}

		prometheusRule.ValidatePrometheusRules(m, errorList)
	} else if !os.IsNotExist(err) {
		errorList.Errorf("reading the 'monitoring' folder failed: %s", err)
	}
}

// applyKubeRBACProxyRule applies KubeRBACProxy rule with optional tracking
func (l *Templates) applyKubeRBACProxyRule(m *module.Module, moduleName *string, errorList *errors.LintRuleErrorsList) {
	if moduleName != nil {
		// With tracking
		trackedKubeRBACProxyRule := exclusions.NewTrackedStringRuleForModule(
			l.cfg.ExcludeRules.KubeRBACProxy.Get(),
			l.tracker,
			ID,
			"kube-rbac-proxy",
			*moduleName,
		)
		rules.NewKubeRbacProxyRuleTracked(trackedKubeRBACProxyRule).
			NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), errorList)
	} else {
		// Without tracking
		rules.NewKubeRbacProxyRule(l.cfg.ExcludeRules.KubeRBACProxy.Get()).
			NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), errorList)
	}
}

// applyServicePortAndPrometheusRules applies service port and prometheus rules with optional tracking
func (l *Templates) applyServicePortAndPrometheusRules(m *module.Module, moduleName *string, errorList *errors.LintRuleErrorsList) {
	var servicePortRule any
	prometheusRule := rules.NewPrometheusRule()

	if moduleName != nil {
		// With tracking
		trackedServicePortRule := exclusions.NewTrackedServicePortRuleForModule(
			l.cfg.ExcludeRules.ServicePort.Get(),
			l.tracker,
			ID,
			"service-port",
			*moduleName,
		)
		servicePortRule = rules.NewServicePortRuleTracked(trackedServicePortRule)
	} else {
		// Without tracking
		servicePortRule = rules.NewServicePortRule(l.cfg.ExcludeRules.ServicePort.Get())
	}

	for _, object := range m.GetStorage() {
		switch rule := servicePortRule.(type) {
		case *rules.ServicePortRuleTracked:
			rule.ObjectServiceTargetPort(object, errorList)
		case *rules.ServicePortRule:
			rule.ObjectServiceTargetPort(object, errorList)
		}
		prometheusRule.PromtoolRuleCheck(m, object, errorList)
	}
}

func (l *Templates) Name() string {
	return l.name
}

func (l *Templates) Desc() string {
	return l.desc
}

func dirExists(modulePath string, path ...string) error {
	searchPath := filepath.Join(append([]string{modulePath}, path...)...)
	_, err := os.Stat(searchPath)
	if err != nil {
		return err
	}
	return nil
}
