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
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/templates/rules"
)

const (
	ID = "templates"
)

// Templates linter
type Templates struct {
	name, desc string
	cfg        *pkg.TemplatesLinterConfig
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.TemplatesLinterConfig, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates",
		cfg:       cfg,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Templates) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// VPA
	rules.NewVPARule(l.cfg.ExcludeRules.VPAAbsent.Get()).ControllerMustHaveVPA(m, errorList.WithMaxLevel(l.cfg.Rules.VPARule.GetLevel()))
	// PDB
	rules.NewPDBRule(l.cfg.ExcludeRules.PDBAbsent.Get()).ControllerMustHavePDB(m, errorList.WithMaxLevel(l.cfg.Rules.PDBRule.GetLevel()))
	// Ingress
	ingressRule := rules.NewIngressRule(l.cfg.ExcludeRules.Ingress.Get())

	// monitoring
	prometheusRule := rules.NewPrometheusRule(l.cfg)
	grafanaRule := rules.NewGrafanaRule(l.cfg)

	if err := dirExists(m.GetPath(), "monitoring"); err == nil {
		grafanaRule.ValidateGrafanaDashboards(m, errorList)
		prometheusRule.ValidatePrometheusRules(m, errorList)
	} else if !os.IsNotExist(err) {
		errorList.Errorf("reading the 'monitoring' folder failed: %s", err)
	}

	rules.NewKubeRbacProxyRule(l.cfg.ExcludeRules.KubeRBACProxy.Get()).
		NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), errorList.WithMaxLevel(l.cfg.Rules.KubeRBACProxyRule.GetLevel()))

	servicePortRule := rules.NewServicePortRule(l.cfg.ExcludeRules.ServicePort.Get())

	for _, object := range m.GetStorage() {
		servicePortRule.ObjectServiceTargetPort(object, errorList.WithMaxLevel(l.cfg.Rules.ServicePortRule.GetLevel()))
		prometheusRule.PromtoolRuleCheck(m, object, errorList.WithMaxLevel(l.cfg.Rules.PrometheusRule.GetLevel()))
		ingressRule.CheckSnippetsRule(object, errorList.WithMaxLevel(l.cfg.Rules.IngressRule.GetLevel()))
	}

	// Cluster domain rule
	clusterDomainRule := rules.NewClusterDomainRule()
	clusterDomainRule.ValidateClusterDomainInTemplates(m, errorList.WithMaxLevel(l.cfg.Rules.ClusterDomainRule.GetLevel()))

	// werf file
	// The following line is commented out because the Werf rule validation is not currently required.
	// If needed in the future, uncomment and ensure the rule is properly configured.
	// rules.NewWerfRule().ValidateWerfTemplates(m, errorList.WithMaxLevel(l.cfg.Rules.WerfRule.GetLevel()))

	rules.NewRegistryRule().CheckRegistrySecret(m, errorList.WithMaxLevel(l.cfg.Rules.RegistryRule.GetLevel()))
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
