package object

import (
	"github.com/deckhouse/d8-lint/internal/module"
	"github.com/deckhouse/d8-lint/pkg/config"
	"github.com/deckhouse/d8-lint/pkg/errors"
)

const (
	ID = "object"
)

// Object linter
type Object struct {
	name, desc string
	cfg        *config.ObjectSettings
}

func New(cfg *config.ObjectSettings) *Object {
	return &Object{
		name: "object",
		desc: "Lint objects",
		cfg:  cfg,
	}
}

func (*Object) Run(m *module.Module) (result errors.LintRuleErrorsList, err error) {
	if m == nil {
		return result, err
	}

	for _, object := range m.GetStorage() {
		result.Merge(applyContainerRules(object))
	}

	return result, nil
}

func (o *Object) Name() string {
	return o.name
}

func (o *Object) Desc() string {
	return o.desc
}
