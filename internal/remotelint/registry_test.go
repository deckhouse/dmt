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
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryAuthPrecedence pins the order credentials are resolved in: the flags
// win over the environment, and each field falls back on its own so a CI job can keep
// the login in the pipeline definition and the password in a secret.
//
// DOCKER_CONFIG points at an empty directory throughout, so the Docker config holds
// nothing and the "neither" case is anonymous on any machine — otherwise a developer
// logged into the registry would see their own credentials answer instead.
func TestRegistryAuthPrecedence(t *testing.T) {
	const registryHost = "registry.example.com"

	for name, tc := range map[string]struct {
		login, password       string
		envLogin, envPassword string
		wantUser, wantPass    string
	}{
		"flags win over the environment": {
			login: "from-flag", password: "flag-secret",
			envLogin: "from-env", envPassword: "env-secret",
			wantUser: "from-flag", wantPass: "flag-secret",
		},
		"the environment answers when no flag was given": {
			envLogin: "from-env", envPassword: "env-secret",
			wantUser: "from-env", wantPass: "env-secret",
		},
		"the login comes from a flag and the password from a secret": {
			login:    "license-token",
			envLogin: "ignored", envPassword: "env-secret",
			wantUser: "license-token", wantPass: "env-secret",
		},
		"neither is anonymous": {},
	} {
		t.Run(name, func(t *testing.T) {
			// t.Setenv forbids t.Parallel here.
			t.Setenv("DOCKER_CONFIG", t.TempDir())
			t.Setenv(loginEnv, tc.envLogin)
			t.Setenv(passwordEnv, tc.envPassword)

			auth := registryAuth(registryHost, tc.login, tc.password)

			if tc.wantUser == "" {
				assert.Equal(t, authn.Anonymous, auth)

				return
			}

			cfg, err := auth.Authorization()
			require.NoError(t, err)

			assert.Equal(t, tc.wantUser, cfg.Username)
			assert.Equal(t, tc.wantPass, cfg.Password)
		})
	}
}
