package matrix

import (
	"fmt"

	"github.com/sourcegraph/conc/pool"

	"github.com/deckhouse/d8-lint/internal/k8s"
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/internal/storage"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
	"github.com/deckhouse/d8-lint/pkg/linters/matrix/rules/modules"
)

// Matrix linter
type Matrix struct {
	name, desc string
	cfg        *config.MatrixSettings
}

func New(cfg *config.MatrixSettings) *Matrix {
	return &Matrix{
		name: "matrix",
		desc: "Matrix check a group of tests to module",
		cfg:  cfg,
	}
}

func (*Matrix) Run(m *module.Module) (errors.LintRuleErrorsList, error) {
	var result errors.LintRuleErrorsList

	values, err := k8s.ComposeValuesFromSchemas(m)
	if err != nil {
		return result, fmt.Errorf("saving values from openapi: %w", err)
	}

	var ch = make(chan errors.LintRuleErrorsList)
	go func() {
		var g = pool.New().WithErrors()
		g.Go(func() error {
			ch <- modules.LintModuleStructure(m.GetPath())
			return nil
		})

		g.Go(func() error {
			objectStore := storage.NewUnstructuredObjectStore()
			err = k8s.RunRender(m, values, objectStore)
			if err != nil {
				return err
			}

			ch <- ApplyLintRules(m, objectStore)

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

func (o *Matrix) Name() string {
	return o.name
}

func (o *Matrix) Desc() string {
	return o.desc
}
