/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remotelint

import (
	"cmp"
	"log/slog"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/pkg/log"
	regclient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

// Environment variables the registry credentials can come from, so a CI job need not
// put a secret on the command line, where it lands in the process list and in the
// job's own command echo.
const (
	loginEnv    = "DMT_REGISTRY_LOGIN"
	passwordEnv = "DMT_REGISTRY_PASSWORD"
)

func newRegistryClient(registryHost, login, password string) *regclient.Client {
	return regclient.New(registryHost, regclient.WithAuth(registryAuth(registryHost, login, password)))
}

// registryAuth resolves credentials for the source registry: the explicit
// login/password first, then DMT_REGISTRY_LOGIN/DMT_REGISTRY_PASSWORD, then the
// Docker config, then anonymous. Each field falls back on its own, so a CI job can
// keep the login in the pipeline definition and the password in a secret.
func registryAuth(registryHost, login, password string) authn.Authenticator {
	login = cmp.Or(login, os.Getenv(loginEnv))
	password = cmp.Or(password, os.Getenv(passwordEnv))

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
// (~/.docker/config.json, written by `d8 dk cr login`). ok is false when the config
// holds no usable entry for the host.
func dockerConfigAuth(registryHost string) (authn.Authenticator, bool) {
	ref, err := name.ParseReference(registryHost)
	if err != nil {
		return nil, false
	}

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
