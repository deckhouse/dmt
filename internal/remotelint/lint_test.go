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
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCutTagFromImagePath(t *testing.T) {
	repository, tag, err := cutTagFromImagePath("registry.example.com/deckhouse/my-module:v0.0.1")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com/deckhouse/my-module", repository)
	require.Equal(t, "v0.0.1", tag)

	// A digest names one manifest, so the release image's tag cannot be derived.
	_, _, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module@sha256:1234567890")
	require.ErrorContains(t, err, "digest not supported")

	_, _, err = cutTagFromImagePath("registry.example.com/deckhouse/my-module")
	require.ErrorContains(t, err, "tag not found in image path")

	repository, tag, err = cutTagFromImagePath("registry.example.com:8080/deckhouse/my-module:v0.0.1")
	require.NoError(t, err)
	require.Equal(t, "registry.example.com:8080/deckhouse/my-module", repository)
	require.Equal(t, "v0.0.1", tag)

	// A reference with no registry of its own would be normalized to Docker Hub, and
	// the credentials would go to a registry the caller never named. The last one is
	// the worst of the three: its port is read as a tag and its host as a repository.
	for _, imagePath := range []string{
		"deckhouse/my-module:v0.0.1",
		"my-module:v1",
		"registry.example.com:5000",
	} {
		_, _, err = cutTagFromImagePath(imagePath)
		require.ErrorContains(t, err, "registry not found in image path", imagePath)
	}
}

// TestExtractLinks covers the two entry kinds whose paths are resolved against
// something other than the entry itself: a symlink resolves against the directory it
// sits in, a hardlink against the archive root. Getting either boundary wrong rejects
// or misplaces links that are perfectly legal.
func TestExtractLinks(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, extract(context.Background(), tarball(t,
		dirEntry("docs"),
		fileEntry("docs/README.md", "hello"),
		symlinkEntry("docs/README.ru.md", "README.md"),
		hardlinkEntry("docs/COPY.md", "docs/README.md"),
	), dir))

	target, err := os.Readlink(filepath.Join(dir, "docs", "README.ru.md"))
	require.NoError(t, err)
	require.Equal(t, "README.md", target)

	content, err := os.ReadFile(filepath.Join(dir, "docs", "COPY.md"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))
}

func TestExtractRejectsEscapes(t *testing.T) {
	for name, entry := range map[string]*tar.Header{
		"parent path":     fileEntry("../evil", "x"),
		"absolute path":   fileEntry("/evil", "x"),
		"symlink escape":  symlinkEntry("link", "../../evil"),
		"hardlink escape": hardlinkEntry("hard", "../evil"),
	} {
		t.Run(name, func(t *testing.T) {
			err := extract(context.Background(), tarball(t, entry), t.TempDir())
			require.ErrorContains(t, err, "escapes output directory")
		})
	}
}

// TestExtractRejectsASymlinkChain is the escape a name check cannot see: both links
// below resolve inside the archive when read as text, but once they are on disk the
// second one is followed through the first, so the file written through them lands
// beside the extraction root instead of inside it. Only resolving each path component
// against what is already on disk — what os.Root does — catches that.
func TestExtractRejectsASymlinkChain(t *testing.T) {
	outside := t.TempDir()
	root := filepath.Join(outside, "root")
	require.NoError(t, os.Mkdir(root, 0o700))

	err := extract(t.Context(), tarball(t,
		symlinkEntry("d1", "."),
		symlinkEntry("d1/d2", ".."),
		fileEntry("d1/d2/pwned", "x"),
	), root)

	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(outside, "pwned"))
}

// TestExtractReportsATruncatedStream covers a producer that fails part-way through:
// the tar writer's deferred Close puts a valid archive terminator into the stream
// before the failure is recorded, so tr.Next sees a clean io.EOF and the extraction
// looks complete. Without the drain at the end of extract, a registry outage is
// reported as a module missing its files.
func TestExtractReportsATruncatedStream(t *testing.T) {
	pr, pw := io.Pipe()

	go func() {
		tw := tar.NewWriter(pw)
		_ = tw.WriteHeader(dirEntry("docs"))
		_ = tw.Close()
		_ = pw.CloseWithError(errors.New("registry went away"))
	}()

	require.ErrorContains(t, extract(t.Context(), pr, t.TempDir()), "registry went away")
}

// TestExtractRejectsLinksThatResolveOutside covers the escapes that reading the
// names as text cannot see. Both chains below place a link inside the root whose
// resolution leaves it, and os.Root catches neither: nothing is written through the
// link, so the guard has to reject it here — the linters read the extracted tree
// afterwards with plain os calls.
func TestExtractRejectsLinksThatResolveOutside(t *testing.T) {
	// "escaping" is a directory beside the root holding the file a link must not
	// reach; each chain is followed by the path that would read it.
	for name, chain := range map[string]struct {
		entries []*tar.Header
		read    string
	}{
		// "sub/toroot" resolves to the root itself, so the second link is created
		// beside the root instead of inside "sub".
		"a link to the root's parent": {
			entries: []*tar.Header{
				dirEntry("sub"),
				symlinkEntry("sub/toroot", ".."),
				symlinkEntry("sub/toroot/up", ".."),
			},
			read: filepath.Join("up", "escaping", "secret.md"),
		},
		// Clean("a/../x") is "x", so every check that cleans the target first calls
		// this one local — while the kernel resolves "a" and only then the "..".
		"a target whose .. is cancelled by a symlink": {
			entries: []*tar.Header{
				symlinkEntry("a", "."),
				symlinkEntry("b", "a/../escaping"),
			},
			read: filepath.Join("b", "secret.md"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			outside := t.TempDir()
			root := filepath.Join(outside, "root")
			require.NoError(t, os.MkdirAll(filepath.Join(outside, "escaping"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(outside, "escaping", "secret.md"), []byte("secret"), 0o600))
			require.NoError(t, os.Mkdir(root, 0o700))

			require.ErrorContains(t, extract(t.Context(), tarball(t, chain.entries...), root),
				"escapes output directory")

			_, err := os.ReadFile(filepath.Join(root, chain.read))
			require.Error(t, err, "a link planted inside the root must not resolve out of it")
		})
	}
}

func dirEntry(name string) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755}
}

func fileEntry(name, content string) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, Size: int64(len(content)), Linkname: content}
}

func symlinkEntry(name, target string) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeSymlink, Name: name, Mode: 0o777, Linkname: target}
}

func hardlinkEntry(name, target string) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeLink, Name: name, Mode: 0o644, Linkname: target}
}

// tarball builds an archive from headers. A regular entry carries its content in
// Linkname, which is unused for that type and saves a second parameter everywhere.
func tarball(t *testing.T, headers ...*tar.Header) io.ReadCloser {
	t.Helper()

	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	for _, hdr := range headers {
		content := ""
		if hdr.Typeflag == tar.TypeReg {
			content, hdr.Linkname = hdr.Linkname, ""
		}

		require.NoError(t, tw.WriteHeader(hdr))

		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())

	return io.NopCloser(buf)
}
