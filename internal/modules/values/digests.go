/*
Copyright 2026 Flant JSC

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

package values

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// dummyDigest is the placeholder image digest injected for every image so
// helm_lib_module_image resolves to a stable, well-formed reference during
// offline rendering. It is a syntactically valid sha256 digest.
const dummyDigest = "sha256:d478cd82cb6a604e3a27383daf93637326d402570b2f3bec835d1f84c9ed0acc"

// imagesDirName is the module subdirectory holding one directory per image.
const imagesDirName = "images"

// loadDigests scans the module's images/ directory and returns a placeholder
// digest for every image (one subdirectory per image), keyed by image name.
// A missing images/ directory yields an empty map — the module ships no images.
func loadDigests(modulePath string) (map[string]any, error) {
	digests := map[string]any{}

	entries, err := os.ReadDir(filepath.Join(modulePath, imagesDirName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return digests, nil
		}

		return nil, fmt.Errorf("read images dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// deckhouse's build keys image digests by the camelCase image name
		// (`last | camelcase | untitle`), which is how templates reference them:
		// dir "pod-reloader" -> helm_lib_module_image (list . "podReloader").
		// Key by that camelCase name so the real helm_lib helper resolves the
		// digest; also keep the raw directory name so templates that reference an
		// image by its verbatim (e.g. underscore) name still render — both are the
		// same placeholder digest, so the extra key is harmless.
		digests[ModuleCamelName(entry.Name())] = dummyDigest
		digests[entry.Name()] = dummyDigest
	}

	return digests, nil
}

// commonImageDirs are the platform "common" images (deckhouse's modules/000-common/
// images) that modules reference via helm_lib_module_common_image. They do not live
// in the linted module, so dmt cannot scan them and must stub them. Strict rendering
// fails on an unstubbed common image, so keep this list roughly in sync with
// deckhouse's common images. It is stub data (dummy digests), not a real digest set.
var commonImageDirs = []string{
	"kube-rbac-proxy", "init", "container", "pause", "coredns",
	"check-kernel-version", "cni-migration-controller", "cni-migration-init-checker",
	"vxlan-offloading-fixer", "debug-container", "redis-static", "nginx-static",
	"shell-operator", "iptables-wrapper", "distroless", "kubernetes", "crane",
	"promtool", "promu", "relocate", "src-artifact", "wheel-artifact", "task",
	"candi", "alt-p11", "csi-external-attacher", "csi-external-provisioner",
	"csi-external-resizer", "csi-external-snapshotter", "csi-livenessprobe",
	"csi-node-driver-registrar", "csi-vsphere-syncer",
}

// commonDigests returns placeholder digests for the platform common images, keyed
// both by camelCase name (how templates reference them, e.g. "kubeRbacProxy") and by
// the raw directory name, so helm_lib_module_common_image resolves offline.
func commonDigests() map[string]any {
	digests := make(map[string]any, len(commonImageDirs)*2)
	for _, name := range commonImageDirs {
		digests[ModuleCamelName(name)] = dummyDigest
		digests[name] = dummyDigest
	}

	return digests
}
