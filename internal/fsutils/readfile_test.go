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

package fsutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile_ReadsNormalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.txt")

	want := []byte("hello")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadFile_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := f.Truncate(MaxLintableFileSize + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := ReadFile(path)
	if err == nil {
		t.Fatalf("expected an error for an oversized file, got %d bytes", len(got))
	}

	if !IsFileTooLarge(err) {
		t.Errorf("expected IsFileTooLarge to report true for %v", err)
	}

	if got != nil {
		t.Errorf("expected no data for an oversized file, got %d bytes", len(got))
	}
}
