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

package schemas

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"strings"
)

// catalog holds the compressed schema catalog compiled into the dmt binary. It
// is produced by tools/schemagen (see scripts/gen-schemas.sh) from the
// datree/crds-catalog repository and the upstream kubernetes JSON schemas, so
// dmt can validate rendered manifests against their schemas fully offline.
//
// The archive is a gzip-compressed tar whose entries are named
//
//	<source>/<kind>__<group>__<version>.json
//
// where <source> is either "crd" (third-party CustomResourceDefinitions from
// the datree catalog, keyed by their full API group) or "k8s" (built-in
// Kubernetes types, keyed by the first DNS label of their API group, empty for
// the core group). Names are lower-cased. This layout lets the runtime look a
// schema up by resource GVK without parsing upstream file-name conventions.
//
//go:embed data/schemas.tar.gz
var catalog []byte

const (
	sourceK8s = "k8s"
	sourceCRD = "crd"
)

// entryKey builds a catalog map key from a schema source and a lookup key.
func entryKey(source, lookup string) string {
	return source + "/" + lookup + ".json"
}

// extractCatalog decompresses the embedded archive in a single streaming pass,
// keeping only the entries whose name is present in want. The full catalog is
// large when decompressed (hundreds of MB of self-contained schemas), so we
// never materialize all of it: a module references only a handful of kinds.
//
// It returns an empty map when no catalog was embedded (the build-time
// generator was never run), so validation degrades to "no schema found"
// instead of failing.
func extractCatalog(want map[string]struct{}) (map[string][]byte, error) {
	out := make(map[string][]byte, len(want))

	if len(want) == 0 || len(bytes.TrimSpace(catalog)) == 0 {
		return out, nil
	}

	gz, err := gzip.NewReader(bytes.NewReader(catalog))
	if err != nil {
		return nil, fmt.Errorf("open schema catalog: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	remaining := len(want)

	for remaining > 0 {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read schema catalog: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, ".json") {
			continue
		}

		if _, ok := want[hdr.Name]; !ok {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read schema %q: %w", hdr.Name, err)
		}

		out[hdr.Name] = data
		remaining--
	}

	return out, nil
}
