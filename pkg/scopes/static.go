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

package scopes

import (
	"github.com/deckhouse/dmt/internal/modules"
	"github.com/deckhouse/dmt/internal/set"
	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
	"github.com/deckhouse/dmt/pkg/linters/container"
	containerrules "github.com/deckhouse/dmt/pkg/linters/container/rules"
	"github.com/deckhouse/dmt/pkg/linters/docs"
	docsrules "github.com/deckhouse/dmt/pkg/linters/docs/rules"
	"github.com/deckhouse/dmt/pkg/linters/hooks"
	hooksrules "github.com/deckhouse/dmt/pkg/linters/hooks/rules"
	"github.com/deckhouse/dmt/pkg/linters/images"
	imagesrules "github.com/deckhouse/dmt/pkg/linters/images/rules"
	moduleLinter "github.com/deckhouse/dmt/pkg/linters/module"
	modulerules "github.com/deckhouse/dmt/pkg/linters/module/rules"
	no_cyrillic "github.com/deckhouse/dmt/pkg/linters/no-cyrillic"
	nocyrillicrules "github.com/deckhouse/dmt/pkg/linters/no-cyrillic/rules"
	"github.com/deckhouse/dmt/pkg/linters/openapi"
	openapirules "github.com/deckhouse/dmt/pkg/linters/openapi/rules"
	"github.com/deckhouse/dmt/pkg/linters/rbac"
	rbacrules "github.com/deckhouse/dmt/pkg/linters/rbac/rules"
	"github.com/deckhouse/dmt/pkg/linters/templates"
	templatesrules "github.com/deckhouse/dmt/pkg/linters/templates/rules"
)

// staticRules is the rule membership of the static scope: for each linter, the IDs of the
// rules this scope asks it for. A rule missing from a linter's set does not run.
//
// static reads the whole committed source tree, which carries every file the rules look
// for, so today it asks for all of them. Nothing enforces that, on purpose: a rule that
// belongs to a built image and not to the source tree is exactly what scopes exist to
// express, so this table is the authority and is not checked against what a linter carries.
// The cost is that a rule added to a linter and forgotten here silently does not run.
//
// Sets are keyed by linter, so rule IDs that repeat across linters — bilingual in docs and
// openapi, werf and mount-points in images/templates/container, ingress in hooks and
// templates — need no qualification here.
var staticRules = map[string]set.Set{
	container.ID: set.New(
		containerrules.APIVersionRuleName,
		containerrules.CheckReadOnlyRootFilesystemRuleName,
		containerrules.ContainerImageNameRuleName,
		containerrules.ContainerSecurityContextRuleName,
		containerrules.ControllerSecurityContextRuleName,
		containerrules.DNSPolicyRuleName,
		containerrules.EnvVariablesDuplicatesRuleName,
		containerrules.HostNetworkPortsRuleName,
		containerrules.ImageDigestRuleName,
		containerrules.ImagePullPolicyRuleName,
		containerrules.LivenessRuleName,
		containerrules.MountPointsRuleName,
		containerrules.NameDuplicatesRuleName,
		containerrules.NamespaceLabelsRuleName,
		containerrules.NoNewPrivilegesRuleName,
		containerrules.PortsRuleName,
		containerrules.PriorityClassRuleName,
		containerrules.ReadinessRuleName,
		containerrules.RecommendedLabelsRuleName,
		containerrules.ResourcesRuleName,
		containerrules.RevisionHistoryLimitRuleName,
		containerrules.SeccompProfileRuleName,
		containerrules.SysCgroupMountRuleName,
	),
	docs.ID: set.New(
		docsrules.BilingualRuleName,
		docsrules.CyrillicInEnglishRuleName,
		docsrules.FrontMatterRuleName,
		docsrules.MarkdownlintRuleName,
		docsrules.NoLangKeyRuleName,
		docsrules.ReadmeRuleName,
		docsrules.SizeRuleName,
	),
	hooks.ID: set.New(
		hooksrules.IngressRuleName,
	),
	images.ID: set.New(
		imagesrules.DistrolessRuleName,
		imagesrules.DockerfileRuleName,
		imagesrules.PatchesRuleName,
		imagesrules.WerfRuleName,
	),
	moduleLinter.ID: set.New(
		modulerules.ConversionsRuleName,
		modulerules.DefinitionFileRuleName,
		modulerules.EnabledScriptRuleName,
		modulerules.HelmignoreRuleName,
		modulerules.LegacyReleaseFileRuleName,
		modulerules.LicenseRuleName,
		modulerules.ModulePackageConsistencyRuleName,
		modulerules.OSSRuleName,
		modulerules.PackageYAMLRuleName,
		modulerules.RequirementsRuleName,
	),
	no_cyrillic.ID: set.New(
		nocyrillicrules.FilesRuleName,
	),
	openapi.ID: set.New(
		openapirules.BilingualRuleName,
		openapirules.CRDsRuleName,
		openapirules.DeckhouseValidationsRuleName,
		openapirules.DocRuYAMLRuleName,
		openapirules.EnumRuleName,
		openapirules.HARuleName,
		openapirules.KeysRuleName,
	),
	rbac.ID: set.New(
		rbacrules.BindingSubjectRuleName,
		rbacrules.PlacementRuleName,
		rbacrules.UserAuthZRuleName,
		rbacrules.WildcardsRuleName,
	),
	templates.ID: set.New(
		templatesrules.CRDEnabledModulesRuleName,
		templatesrules.ClusterDomainRuleName,
		templatesrules.EnabledModulesRuleName,
		templatesrules.GrafanaRuleName,
		templatesrules.HTTPRouteRuleName,
		templatesrules.HelmRenderRuleName,
		templatesrules.IngressRuleName,
		templatesrules.KubeRbacProxyRuleName,
		templatesrules.MountPointsRuleName,
		templatesrules.OpenAPIValuesQuoteRuleName,
		templatesrules.PDBRuleName,
		// prometheus-rules covers two distinct rules — NewPrometheusRule over
		// monitoring/prometheus-rules files and NewPromtoolRule over rendered objects.
		// They share this ID, so a scope can only ask for both or neither.
		templatesrules.PrometheusRuleName,
		templatesrules.RegistryRuleName,
		templatesrules.ServicePortRuleName,
		templatesrules.VPARuleName,
		templatesrules.WebhookConfigurationRuleName,
		templatesrules.WerfRuleName,
	),
}

// staticLinters builds the linters of the static scope, handing each one its slice of the
// module config and the rule IDs staticRules asks it for.
func staticLinters(m *modules.Module, errList *errors.LintRuleErrorsList) []Linter {
	cfg := m.GetModuleConfig()
	if cfg == nil {
		// A module whose config never loaded still has to yield a linter list. The zero
		// settings leave every impact unset, which WithMaxLevel reads as "no cap" — the
		// same thing an absent .dmtlint.yaml produces.
		cfg = &pkg.LintersSettings{}
	}

	return []Linter{
		openapi.New(&cfg.OpenAPI, staticRules[openapi.ID], m, errList),
		no_cyrillic.New(&cfg.NoCyrillic, staticRules[no_cyrillic.ID], m, errList),
		container.New(&cfg.Container, staticRules[container.ID], m, errList),
		templates.New(&cfg.Templates, staticRules[templates.ID], m, errList),
		images.New(&cfg.Image, staticRules[images.ID], m, errList),
		rbac.New(&cfg.RBAC, staticRules[rbac.ID], m, errList),
		hooks.New(&cfg.Hooks, staticRules[hooks.ID], m, errList),
		moduleLinter.New(&cfg.Module, staticRules[moduleLinter.ID], m, errList),
		docs.New(&cfg.Documentation, staticRules[docs.ID], m, errList),
	}
}
