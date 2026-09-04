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

package remote

import (
	"archive/tar"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

// extractImage flattens the image into a temporary directory and returns its path.
// The caller owns the directory and must remove it.
func extractImage(ctx context.Context, image registry.Image) (string, error) {
	tempDir, err := os.MkdirTemp("", "dmt-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	rc := image.Extract()
	defer rc.Close()

	if err = extract(ctx, rc, tempDir); err != nil {
		os.RemoveAll(tempDir)

		return "", fmt.Errorf("failed to extract image: %w", err)
	}

	return tempDir, nil
}

// extract unpacks a tar stream under root, rejecting any entry that would write or
// point outside it.
func extract(ctx context.Context, rc io.ReadCloser, root string) error {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open output directory: %w", err)
	}

	defer dir.Close()

	tr := tar.NewReader(rc)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := tr.Next()
		if stderrors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		name, err := safeName(hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = dir.MkdirAll(name, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("mkdir %q: %w", hdr.Name, err)
			}
		case tar.TypeReg:
			if err = writeRegularFile(dir, name, tr, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("write file %q: %w", hdr.Name, err)
			}
		case tar.TypeSymlink:
			// dir stops a link from being followed out of the tree, but the linters
			// that read the extracted files afterwards use plain os calls, so a link
			// pointing out of it has to be rejected here as well. A symlink target is
			// resolved by whoever follows it, relative to the directory the link sits
			// in. So the base is that directory while the boundary stays the
			// extraction root — passing the entry itself as the boundary would reject
			// a link to its own sibling.
			if filepath.IsAbs(hdr.Linkname) || !staysWithin(filepath.Dir(name), hdr.Linkname) {
				return fmt.Errorf("symlink %q escapes output directory", hdr.Name)
			}

			if err = dir.Symlink(hdr.Linkname, name); err != nil {
				return fmt.Errorf("create symlink %q: %w", hdr.Name, err)
			}
		case tar.TypeLink:
			// A hardlink names an already-extracted entry by its path within the
			// archive, so it resolves against the root and not against this entry.
			linkName, err := safeName(hdr.Linkname)
			if err != nil {
				return err
			}

			if err = dir.Link(linkName, name); err != nil {
				return fmt.Errorf("create hardlink %q: %w", hdr.Name, err)
			}
		}
	}

	// tr.Next reports io.EOF as soon as it reads the archive terminator, and the
	// producer writes that terminator from a deferred Close before it records its
	// own failure — a registry error mid-pull therefore surfaces only on the next
	// read of the stream. Draining it is what tells a truncated image apart from a
	// module that is genuinely missing half its files.
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("read tar: %w", err)
	}

	return nil
}

// writeRegularFile writes one regular tar entry and limits restored permissions to owner bits.
func writeRegularFile(dir *os.Root, target string, src io.Reader, mode os.FileMode) error {
	if err := dir.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	out, err := dir.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode&0o700)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	if _, err = io.Copy(out, src); err != nil {
		closeErr := out.Close()
		if closeErr != nil {
			return fmt.Errorf("copy file: %w; close file: %v", err, closeErr)
		}

		return fmt.Errorf("copy file: %w", err)
	}

	if err = out.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}

	return nil
}

// safeName cleans a tar entry name into a path relative to the extraction root and
// rejects absolute paths or parent-directory escapes.
func safeName(name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))

	if filepath.IsAbs(clean) || !staysWithin(".", clean) {
		return "", fmt.Errorf("path %q escapes output directory", name)
	}

	return clean, nil
}

// staysWithin reports whether name resolves under the extraction root when
// interpreted relative to base. Both are paths relative to that root.
func staysWithin(base, name string) bool {
	target := filepath.Clean(filepath.Join(base, name))

	return target != ".." && !strings.HasPrefix(target, ".."+string(filepath.Separator))
}
