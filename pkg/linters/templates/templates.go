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
	"github.com/deckhouse/dmt/pkg/config"
	"github.com/deckhouse/dmt/pkg/errors"
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
	moduleCfg  *config.ModuleConfig
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates",
		cfg:       &cfg.LintersSettings.Templates,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Templates.Impact),
		moduleCfg: cfg,
	}
}

func (l *Templates) GetRuleImpact(ruleName string) *pkg.Level {
	if l.moduleCfg != nil {
		return l.moduleCfg.LintersSettings.GetRuleImpact(ID, ruleName)
	}
	return l.cfg.Impact
}

func (l *Templates) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// VPA
	vpaRuleImpact := l.GetRuleImpact("vpa")
	if vpaRuleImpact != nil {
		vpaErrorList := errorList.WithMaxLevel(vpaRuleImpact)
		rules.NewVPARule(l.cfg.ExcludeRules.VPAAbsent.Get()).ControllerMustHaveVPA(m, vpaErrorList)
	} else {
		rules.NewVPARule(l.cfg.ExcludeRules.VPAAbsent.Get()).ControllerMustHaveVPA(m, errorList)
	}

	// PDB
	pdbRuleImpact := l.GetRuleImpact("pdb")
	if pdbRuleImpact != nil {
		pdbErrorList := errorList.WithMaxLevel(pdbRuleImpact)
		rules.NewPDBRule(l.cfg.ExcludeRules.PDBAbsent.Get()).ControllerMustHavePDB(m, pdbErrorList)
	} else {
		rules.NewPDBRule(l.cfg.ExcludeRules.PDBAbsent.Get()).ControllerMustHavePDB(m, errorList)
	}

	// Ingress
	ingressRule := rules.NewIngressRule(l.cfg.ExcludeRules.Ingress.Get())
	ingressRuleImpact := l.GetRuleImpact("ingress")

	// monitoring
	prometheusRule := rules.NewPrometheusRule(l.cfg)
	grafanaRule := rules.NewGrafanaRule(l.cfg)
	prometheusRuleImpact := l.GetRuleImpact("prometheus")
	grafanaRuleImpact := l.GetRuleImpact("grafana")

	if err := dirExists(m.GetPath(), "monitoring"); err == nil {
		if grafanaRuleImpact != nil {
			grafanaErrorList := errorList.WithMaxLevel(grafanaRuleImpact)
			grafanaRule.ValidateGrafanaDashboards(m, grafanaErrorList)
		} else {
			grafanaRule.ValidateGrafanaDashboards(m, errorList)
		}

		if prometheusRuleImpact != nil {
			prometheusErrorList := errorList.WithMaxLevel(prometheusRuleImpact)
			prometheusRule.ValidatePrometheusRules(m, prometheusErrorList)
		} else {
			prometheusRule.ValidatePrometheusRules(m, errorList)
		}
	} else if !os.IsNotExist(err) {
		errorList.Errorf("reading the 'monitoring' folder failed: %s", err)
	}

	// KubeRBACProxy
	kubeRbacProxyRuleImpact := l.GetRuleImpact("kube-rbac-proxy")
	if kubeRbacProxyRuleImpact != nil {
		kubeRbacProxyErrorList := errorList.WithMaxLevel(kubeRbacProxyRuleImpact)
		rules.NewKubeRbacProxyRule(l.cfg.ExcludeRules.KubeRBACProxy.Get()).NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), kubeRbacProxyErrorList)
	} else {
		rules.NewKubeRbacProxyRule(l.cfg.ExcludeRules.KubeRBACProxy.Get()).NamespaceMustContainKubeRBACProxyCA(m.GetObjectStore(), errorList)
	}

	servicePortRule := rules.NewServicePortRule(l.cfg.ExcludeRules.ServicePort.Get())
	servicePortRuleImpact := l.GetRuleImpact("service-port")

	for _, object := range m.GetStorage() {
		if servicePortRuleImpact != nil {
			servicePortErrorList := errorList.WithMaxLevel(servicePortRuleImpact)
			servicePortRule.ObjectServiceTargetPort(object, servicePortErrorList)
		} else {
			servicePortRule.ObjectServiceTargetPort(object, errorList)
		}

		if prometheusRuleImpact != nil {
			prometheusErrorList := errorList.WithMaxLevel(prometheusRuleImpact)
			prometheusRule.PromtoolRuleCheck(m, object, prometheusErrorList)
		} else {
			prometheusRule.PromtoolRuleCheck(m, object, errorList)
		}

		if ingressRuleImpact != nil {
			ingressErrorList := errorList.WithMaxLevel(ingressRuleImpact)
			ingressRule.CheckSnippetsRule(object, ingressErrorList)
		} else {
			ingressRule.CheckSnippetsRule(object, errorList)
		}
	}

	// Cluster domain rule
	clusterDomainRule := rules.NewClusterDomainRule()
	clusterDomainRuleImpact := l.GetRuleImpact("cluster-domain")
	if clusterDomainRuleImpact != nil {
		clusterDomainErrorList := errorList.WithMaxLevel(clusterDomainRuleImpact)
		clusterDomainRule.ValidateClusterDomainInTemplates(m, clusterDomainErrorList)
	} else {
		clusterDomainRule.ValidateClusterDomainInTemplates(m, errorList)
	}

	// Registry rule
	registryRuleImpact := l.GetRuleImpact("registry")
	if registryRuleImpact != nil {
		registryErrorList := errorList.WithMaxLevel(registryRuleImpact)
		rules.NewRegistryRule().CheckRegistrySecret(m, registryErrorList)
	} else {
		// werf file
		// The following line is commented out because the Werf rule validation is not currently required.
		// If needed in the future, uncomment and ensure the rule is properly configured.
		// rules.NewWerfRule().ValidateWerfTemplates(m, errorList)
		rules.NewRegistryRule().CheckRegistrySecret(m, errorList)
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
