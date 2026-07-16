package linters

import "github.com/deckhouse/dmt/pkg"

type RemoteLinter interface {
	RunRemote(cfg *LinterConfig)
	Name() string
}

type LinterConfig struct {
	Name      string
	Namespace string
	Path      string

	LinterSettings *pkg.LintersSettings
}
