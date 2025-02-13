package templates

import (
	"github.com/deckhouse/dmt/internal/module"
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
}

func New(cfg *config.ModuleConfig, errorList *errors.LintRuleErrorsList) *Templates {
	return &Templates{
		name:      ID,
		desc:      "Lint templates",
		cfg:       &cfg.LintersSettings.Templates,
		ErrorList: errorList.WithLinterID(ID).WithMaxLevel(cfg.LintersSettings.Templates.Impact),
	}
}

func (l *Templates) Run(m *module.Module) {
	if m == nil {
		return
	}

	errorList := l.ErrorList.WithModule(m.GetName())

	// VPA
	rules.NewVPARule(l.cfg.ExcludeRules.VPAAbsent.Get()).ControllerMustHaveVPA(m, errorList)
	// PDB
	pdb := rules.NewPDBRule(l.cfg.ExcludeRules.PDBAbsent.Get())
	pdb.ControllerMustHavePDB(m, errorList)
	pdb.DaemonSetMustNotHavePDB(m, errorList)

	servicePortRule := rules.NewServicePortRule(l.cfg.ExcludeRules.ServicePort.Get())
	kubeRbacRule := rules.NewKubeRbacProxyRule(l.cfg.ExcludeRules.KubeRBACProxy.Get())

	for _, object := range m.GetStorage() {
		servicePortRule.ObjectServiceTargetPort(object, errorList)
		kubeRbacRule.NamespaceMustContainKubeRBACProxyCA(object, errorList)
	}
}

func (l *Templates) Name() string {
	return l.name
}

func (l *Templates) Desc() string {
	return l.desc
}
