package remotelint

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

// ExtractImage extracts the image to a temporary directory and returns the path to the directory
func ExtractImage(ctx context.Context, image registry.Image) (string, error) {
	tempDir, err := os.MkdirTemp("", "dmt-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	rc := image.Extract()
	defer rc.Close()

	err = extract(ctx, rc, tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract image: %w", err)
	}

	return tempDir, nil
}

func extract(ctx context.Context, rc io.ReadCloser, target string) error {
	tr := tar.NewReader(rc)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		target, err := safeJoin(target, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("mkdir %q: %w", hdr.Name, err)
			}
		case tar.TypeReg:
			if err = writeRegularFile(target, tr, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("write file %q: %w", hdr.Name, err)
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(hdr.Linkname) || !staysWithin(target, filepath.Dir(target), hdr.Linkname) {
				return fmt.Errorf("symlink %q escapes output directory", hdr.Name)
			}

			if err = os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %q: %w", hdr.Name, err)
			}
		case tar.TypeLink:
			linkTarget, err := safeJoin(target, hdr.Linkname)
			if err != nil {
				return err
			}

			if err = os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("create hardlink %q: %w", hdr.Name, err)
			}
		}
	}

	return nil
}

// writeRegularFile writes one regular tar entry and limits restored permissions to owner bits.
func writeRegularFile(target string, src io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode&0o700)
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

// safeJoin joins name under root and rejects absolute paths or parent-directory escapes.
func safeJoin(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("path %q escapes output directory", name)
	}

	target := filepath.Join(root, name)
	if !staysWithin(root, root, name) {
		return "", fmt.Errorf("path %q escapes output directory", name)
	}

	return target, nil
}

// staysWithin reports whether name resolves under root when interpreted relative to base.
func staysWithin(root, base, name string) bool {
	target := filepath.Clean(filepath.Join(base, name))
	rel, err := filepath.Rel(root, target)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
