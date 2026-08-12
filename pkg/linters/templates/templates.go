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
	"context"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/internal/modules"
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
	module     *modules.Module
	ErrorList  *errors.LintRuleErrorsList
}

func New(cfg *pkg.TemplatesLinterConfig, m *modules.Module, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates",
		cfg:       cfg,
		module:    m,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.Impact),
	}
}

func (l *Templates) Lint(ctx context.Context) {
	if l.module == nil {
		return
	}

	for _, rule := range l.rules() {
		rule.Check(ctx)
	}
}

// rules builds this linter's rule set. Keeping the set as data — rather than a
// sequence of hand-written calls — is what lets rules be selected or grouped
// later without touching the rules themselves.
func (l *Templates) rules() []pkg.Rule {
	m := l.module
	cfg := l.cfg
	errorList := l.ErrorList.WithModule(m.GetName())

	// level scopes errorList to the configured impact of a single rule.
	level := func(rule pkg.RuleConfig) *errors.LintRuleErrorsList {
		return errorList.WithMaxLevel(rule.GetLevel())
	}

	ruleSet := []pkg.Rule{
		rules.NewVPARule(cfg.ExcludeRules.VPAAbsent.Get(), m, level(cfg.Rules.VPARule)),
		rules.NewPDBRule(cfg.ExcludeRules.PDBAbsent.Get(), m, level(cfg.Rules.PDBRule)),
	}

	// The monitoring/ file checks only apply to modules that ship that folder.
	// The promtool check below is deliberately not gated the same way: it runs
	// against rendered objects, which exist regardless of the folder.
	if err := dirExists(m.GetPath(), "monitoring"); err == nil {
		ruleSet = append(ruleSet,
			rules.NewGrafanaRule(cfg, m, level(cfg.Rules.GrafanaRule)),
			rules.NewPrometheusRule(cfg, m, level(cfg.Rules.PrometheusRule)),
		)
	} else if !os.IsNotExist(err) {
		errorList.Errorf("reading the 'monitoring' folder failed: %s", err)
	}

	return append(ruleSet,
		rules.NewKubeRbacProxyRule(cfg.ExcludeRules.KubeRBACProxy.Get(), m, level(cfg.Rules.KubeRBACProxyRule)),
		rules.NewServicePortRule(cfg.ExcludeRules.ServicePort.Get(), m, level(cfg.Rules.ServicePortRule)),
		rules.NewPromtoolRule(cfg, m, level(cfg.Rules.PrometheusRule)),
		rules.NewIngressRule(cfg.ExcludeRules.Ingress.Get(), m, level(cfg.Rules.IngressRule)),
		rules.NewHTTPRouteRule(cfg.ExcludeRules.HTTPRoute.Get(), m, level(cfg.Rules.HTTPRouteRule)),
		rules.NewClusterDomainRule(m, level(cfg.Rules.ClusterDomainRule)),
		// The werf rule has no rule-level config, so it reports at the linter's level.
		rules.NewWerfRule(m, errorList),
		rules.NewRegistryRule(m, level(cfg.Rules.RegistryRule)),
		rules.NewWebhookConfigurationRule(cfg.ExcludeRules.WebhookConfiguration.Get(), m, level(cfg.Rules.WebhookConfigurationRule)),
		rules.NewEnabledModulesRule(
			cfg.ExcludeRules.EnabledModules.Files.Get(),
			cfg.ExcludeRules.EnabledModules.Directories.Get(),
			m, level(cfg.Rules.EnabledModulesRule)),
		rules.NewMountPointsRule(cfg.ExcludeRules.MountPoints.Get(), m, level(cfg.Rules.MountPointsRule)),
		rules.NewHelmRenderRule(m, level(cfg.Rules.HelmRenderRule)),
	)
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
