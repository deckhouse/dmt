package linters

type RemoteBundleLinter interface {
	RunRemoteForBundle(cfg *LinterConfig)
	Name() string
}

type RemoteReleaseLinter interface {
	RunRemoteForRelease(cfg *LinterConfig)
	Name() string
}

type LinterConfig struct {
	Name      string
	Namespace string
	Path      string
}
