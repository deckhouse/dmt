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

package container

import (
	"context"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container/rules"
)

const (
	ID = "container"
)

// Container linter
type Container struct {
	name, desc string
	cfg        *pkg.ContainerLinterConfig
	module     pkg.Module
	ErrorList  *errors.LintRuleErrorsList

	// objects holds the containers extracted from the module once per run; see
	// rules.CollectObjectContainers.
	objects []rules.ObjectContainers
}

func New(containerCfg *pkg.ContainerLinterConfig, m pkg.Module, errorList *errors.LintRuleErrorsList) *Container {
	return &Container{
		name:      ID,
		desc:      "Lint container objects",
		cfg:       containerCfg,
		module:    m,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(containerCfg.Impact),
	}
}

func (l *Container) Lint(ctx context.Context) {
	if l.module == nil {
		return
	}

	// Extract containers once, before building the rule set: the extraction can
	// itself report a finding, and it must do so once per object rather than
	// once per rule. It also applies the gates that decide which rules an object
	// takes part in at all.
	l.objects = rules.CollectObjectContainers(
		l.module.GetStorage(),
		l.ErrorList.WithModule(l.module.GetName()))

	for _, rule := range l.rules() {
		rule.Check(ctx)
	}
}

// rules builds this linter's rule set. Keeping the set as data — rather than
// the two slices of hand-written adapters this linter used to carry — is what
// lets rules be selected or grouped later without touching the rules themselves.
//
// Object rules see every rendered object; container and probe rules see only
// the objects that passed the extraction gates, via l.objects.
func (l *Container) rules() []pkg.Rule {
	m := l.module
	cfg := l.cfg
	objects := l.objects
	errorList := l.ErrorList.WithModule(m.GetName())

	// level scopes errorList to the configured impact of a single rule.
	level := func(rule pkg.RuleConfig) *errors.LintRuleErrorsList {
		return errorList.WithMaxLevel(rule.GetLevel())
	}

	return []pkg.Rule{
		// object-scoped
		rules.NewRecommendedLabelsRule(m, level(cfg.Rules.RecommendedLabelsRule)),
		rules.NewNamespaceLabelsRule(cfg.ExcludeRules.NamespaceLabelsRule.Get(), m, level(cfg.Rules.NamespaceLabelsRule)),
		rules.NewAPIVersionRule(m, level(cfg.Rules.APIVersionRule)),
		rules.NewPriorityClassRule(cfg.ExcludeRules.PriorityClass.Get(), m, level(cfg.Rules.PriorityClassRule)),
		rules.NewDNSPolicyRule(cfg.ExcludeRules.DNSPolicy.Get(), m, level(cfg.Rules.DNSPolicyRule)),
		rules.NewControllerSecurityContextRule(cfg.ExcludeRules.ControllerSecurityContext.Get(), m, level(cfg.Rules.ControllerSecurityContextRule)),
		rules.NewRevisionHistoryLimitRule(m, level(cfg.Rules.NewRevisionHistoryLimitRule)),

		// container-scoped, over every container including init ones
		rules.NewNameDuplicatesRule(objects, level(cfg.Rules.NameDuplicatesRule)),
		rules.NewCheckReadOnlyRootFilesystemRule(cfg.ExcludeRules.ReadOnlyRootFilesystem.Get(), objects, level(cfg.Rules.ReadOnlyRootFilesystemRule)),
		rules.NewNoNewPrivilegesRule(cfg.ExcludeRules.NoNewPrivileges.Get(), objects, level(cfg.Rules.NoNewPrivilegesRule)),
		rules.NewSeccompProfileRule(cfg.ExcludeRules.SeccompProfile.Get(), objects, level(cfg.Rules.SeccompProfileRule)),
		rules.NewHostNetworkPortsRule(cfg.ExcludeRules.HostNetworkPorts.Get(), objects, level(cfg.Rules.HostNetworkPortsRule)),
		rules.NewEnvVariablesDuplicatesRule(objects, level(cfg.Rules.EnvVariablesDuplicatesRule)),
		rules.NewImageDigestRule(cfg.ExcludeRules.ImageDigest.Get(), objects, level(cfg.Rules.ImageDigestRule)),
		rules.NewContainerImageNameRule(cfg.ExcludeRules.ContainerImageName.Get(), objects, level(cfg.Rules.ContainerImageNameRule)),
		rules.NewImagePullPolicyRule(objects, level(cfg.Rules.ImagePullPolicyRule)),
		rules.NewResourcesRule(cfg.ExcludeRules.Resources.Get(), objects, level(cfg.Rules.ResourcesRule)),
		rules.NewContainerSecurityContextRule(cfg.ExcludeRules.SecurityContext.Get(), objects, level(cfg.Rules.ContainerSecurityContextRule)),
		rules.NewPortsRule(cfg.ExcludeRules.Ports.Get(), objects, level(cfg.Rules.PortsRule)),
		rules.NewMountPointsRule(cfg.ExcludeRules.MountPoints.Get(), m.GetPath(), objects, level(cfg.Rules.MountPointsRule)),

		// probe rules, over non-init containers only
		rules.NewLivenessRule(cfg.ExcludeRules.Liveness.Get(), objects, level(cfg.Rules.LivenessRule)),
		rules.NewReadinessRule(cfg.ExcludeRules.Readiness.Get(), objects, level(cfg.Rules.ReadinessRule)),
	}
}

func (l *Container) Name() string {
	return l.name
}

func (l *Container) Desc() string {
	return l.desc
}
