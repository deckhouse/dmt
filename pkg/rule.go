package pkg

type RuleI interface {
	Name() string
	Enabled() error
}
