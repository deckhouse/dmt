package remotelint

import (
	"log/slog"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/pkg/log"
	regclient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

func initRegistryClient(registryHost string, login, password string) *regclient.Client {
	auth := registryAuth(registryHost, login, password)

	return regclient.New(registryHost, regclient.WithAuth(auth))
}

// registryAuth resolves credentials for the source registry, mirroring the
// pre-#386 priority: explicit login/password, then license token, then the
// Docker config, then anonymous.
func registryAuth(registryHost string, login, password string) authn.Authenticator {
	if login != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: login,
			Password: password,
		})
	}

	if auth, ok := dockerConfigAuth(registryHost); ok {
		return auth
	}

	log.Debug("using anonymous access for the source registry", slog.String("registry", registryHost))

	return authn.Anonymous
}

// dockerConfigAuth resolves credentials for registryHost from the Docker config
// (~/.docker/config.json, written by `d8 dk cr login`). ok is false when the
// config holds no usable entry for the host.
func dockerConfigAuth(registryHost string) (authn.Authenticator, bool) {
	ref, err := name.ParseReference(registryHost)
	if err != nil {
		return nil, false
	}

	log.Warn("debug dockerConfigAuth", slog.String(
		"registryHost", registryHost),
		slog.String("ref", ref.String()),
		slog.String("registry", ref.Context().RegistryStr()),
	)

	reg, err := name.NewRegistry(ref.Context().RegistryStr())
	if err != nil {
		return nil, false
	}

	auth, err := authn.DefaultKeychain.Resolve(reg)
	if err != nil || auth == authn.Anonymous {
		return nil, false
	}

	cfg, err := auth.Authorization()
	if err != nil {
		return nil, false
	}

	if cfg.Username == "" && cfg.Password == "" && cfg.Auth == "" && cfg.IdentityToken == "" {
		return nil, false
	}

	log.Debug("using Docker config credentials", slog.String("registry", reg.String()))

	return auth, true
}
