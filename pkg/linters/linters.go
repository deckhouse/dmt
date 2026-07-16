package linters

type RemoteLinter interface {
	RunRemote(cfg *LinterConfig)
	Name() string
}

type LinterConfig struct {
	Name      string
	Namespace string
	Path      string
}
