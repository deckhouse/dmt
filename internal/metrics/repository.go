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

package metrics

import (
	"strings"

	"github.com/deckhouse/dmt/pkg"
)

func getRepositoryAddress(dir string) string {
	return convertToHTTPS(pkg.RepositoryOriginURL(dir))
}

func convertToHTTPS(repoURL string) string {
	if strings.HasPrefix(repoURL, "git@") {
		// Convert SSH format to HTTPS
		repoURL = strings.Replace(repoURL, ":", "/", 1)
		repoURL = strings.Replace(repoURL, "git@", "https://", 1)
		repoURL = strings.TrimSuffix(repoURL, ".git")
	}

	if strings.HasPrefix(repoURL, "https://") && strings.Contains(repoURL, "@") {
		// Remove token from HTTPS URL
		repoURL = strings.Split(repoURL, "@")[1]
		repoURL = strings.TrimSuffix(repoURL, ".git")
		repoURL = "https://" + repoURL
	}

	return repoURL
}
