package resources

import (
	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/resources/pdb"
	rbacproxy "github.com/deckhouse/d8-lint/pkg/linters/resources/rbac-proxy"
	"github.com/deckhouse/d8-lint/pkg/linters/resources/vpa"
)

type Resources struct {
	name, desc string
	cfg        *config.ResourcesSettings
}

func New(cfg *config.ResourcesSettings) *Resources {
	return &Resources{
		name: "resources",
		desc: "Lint resources",
		cfg:  cfg,
	}
}

func (*Resources) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	var ch = make(chan errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithErrors()
		g.Go(func() error {
			ch <- applyLintRules(m)

			return nil
		})
		err = g.Wait()
		close(ch)
	}()

	for er := range ch {
		result.Merge(er)
	}

	return result, err
}

func (o *Resources) Name() string {
	return o.name
}

func (o *Resources) Desc() string {
	return o.desc
}

func applyLintRules(md *module.Module) (result errors.LintRuleErrorsList) {
	result.Merge(vpa.ControllerMustHaveVPA(md))
	result.Merge(pdb.ControllerMustHavePDB(md))
	result.Merge(pdb.DaemonSetMustNotHavePDB(md))
	result.Merge(rbacproxy.NamespaceMustContainKubeRBACProxyCA(md.GetObjectStore()))

	return result
}
