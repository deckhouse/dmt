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

// Command schemagen packs Kubernetes and CustomResourceDefinition JSON schemas
// into the compressed catalog that dmt embeds (internal/schemas/data).
//
// It reads two already-downloaded source trees and normalizes their file names
// into the flat, GVK-addressable layout the runtime expects:
//
//	crd/<kind>__<group>__<version>.json  — from datree/crds-catalog
//	k8s/<kind>__<group>__<version>.json  — from the upstream k8s JSON schemas
//
// where <group> is the full API group for CRDs and the first DNS label of the
// group for built-in types (empty for the core group), all lower-cased.
//
// Usage:
//
//	schemagen -datree <dir> -k8s <dir> -out internal/schemas/data/schemas.tar.gz
//
// See scripts/gen-schemas.sh for how the source trees are fetched.
package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var versionRe = regexp.MustCompile(`^v[0-9]`)

func main() {
	datreeDir := flag.String("datree", "", "path to a checkout of datreeio/crds-catalog")
	k8sDir := flag.String("k8s", "", "path to a directory of standalone kubernetes JSON schemas")
	out := flag.String("out", "internal/schemas/data/schemas.tar.gz", "output archive path")
	flag.Parse()

	entries := map[string][]byte{}

	if *datreeDir != "" {
		n, err := collectDatree(*datreeDir, entries)
		if err != nil {
			log.Fatalf("collect datree schemas: %v", err)
		}

		log.Printf("collected %d CRD schemas from %s", n, *datreeDir)
	}

	if *k8sDir != "" {
		n, err := collectK8s(*k8sDir, entries)
		if err != nil {
			log.Fatalf("collect k8s schemas: %v", err)
		}

		log.Printf("collected %d built-in schemas from %s", n, *k8sDir)
	}

	if len(entries) == 0 {
		log.Fatal("no schemas collected; provide -datree and/or -k8s")
	}

	if err := writeArchive(*out, entries); err != nil {
		log.Fatalf("write archive: %v", err)
	}

	log.Printf("wrote %d schemas to %s", len(entries), *out)
}

// collectDatree walks a datree/crds-catalog checkout. Each CRD schema lives at
// <group>/<kind>_<version>.json.
func collectDatree(root string, entries map[string][]byte) (int, error) {
	count := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Group is the immediate parent directory; skip loose top-level files
		// (README, etc.) and the repo's Utilities/.github directories.
		group := filepath.Dir(rel)
		if group == "." || strings.HasPrefix(group, ".") || strings.HasPrefix(group, "Utilities") {
			return nil
		}

		// Group directories are a single level; ignore anything deeper.
		if strings.Contains(group, string(filepath.Separator)) {
			return nil
		}

		base := strings.TrimSuffix(d.Name(), ".json")

		idx := strings.LastIndex(base, "_")
		if idx <= 0 {
			return nil
		}

		kind, version := base[:idx], base[idx+1:]

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		entries[key("crd", kind, group, version)] = data
		count++

		return nil
	})

	return count, err
}

// collectK8s walks a directory of standalone Kubernetes JSON schemas named
// <kind>[-<group>]-<version>.json (e.g. deployment-apps-v1.json, service-v1.json).
func collectK8s(root string, entries map[string][]byte) (int, error) {
	count := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		base := strings.TrimSuffix(d.Name(), ".json")

		// Skip aggregate/helper documents that are not addressable resources.
		if strings.HasPrefix(base, "_") || base == "all" {
			return nil
		}

		parts := strings.Split(base, "-")
		if len(parts) < 2 {
			return nil // no version segment -> not a versioned resource schema
		}

		version := parts[len(parts)-1]
		if !versionRe.MatchString(version) {
			return nil
		}

		kind := parts[0]

		group := ""
		if len(parts) >= 3 {
			group = strings.Join(parts[1:len(parts)-1], "-")
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		entries[key("k8s", kind, group, version)] = data
		count++

		return nil
	})

	return count, err
}

func key(source, kind, group, version string) string {
	return fmt.Sprintf("%s/%s__%s__%s.json",
		source,
		strings.ToLower(kind),
		strings.ToLower(group),
		strings.ToLower(version),
	)
}

func writeArchive(out string, entries map[string][]byte) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	// Deterministic order keeps the embedded blob reproducible across runs.
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		data := entries[name]

		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(data)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if _, err := tw.Write(data); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return gz.Close()
}
