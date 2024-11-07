package matrix

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	matrixConfig "github.com/deckhouse/d8-lint/pkg/linters/matrix/config"
)

// Matrix linter
type Matrix struct {
	name, desc string
	cfg        *config.MatrixSettings
}

func New(cfg *config.MatrixSettings) *Matrix {
	matrixConfig.Cfg = cfg

	return &Matrix{
		name: "matrix",
		desc: "Matrix check a group of tests to module",
		cfg:  cfg,
	}
}

func (*Matrix) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	//values, err := module.ComposeValuesFromSchemas(m)
	//if err != nil {
	//	return result, fmt.Errorf("saving values from openapi: %w", err)
	//}

	var ch = make(chan *errors.LintRuleErrorsList)
	go func() {
		//var g = pool.New().WithErrors()
		//g.Go(func() error {
		//	ch <- modules.LintModuleStructure(m.GetPath())
		//	return nil
		//})

		//g.Go(func() error {
		//	objectStore := storage.NewUnstructuredObjectStore()
		//	err = module.RunRender(m, values, objectStore)
		//	if err != nil {
		//		return err
		//	}
		//
		//	ch <- ApplyLintRules(m, objectStore)
		//
		//	return nil
		//})
		//err = g.Wait()
		close(ch)
	}()

	for er := range ch {
		result.Merge(*er)
	}

	return result, err
}

func (o *Matrix) Name() string {
	return o.name
}

func (o *Matrix) Desc() string {
	return o.desc
}
