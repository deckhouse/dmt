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

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/dmt/internal/mocks"
	"github.com/deckhouse/dmt/pkg/errors"
)

// TestFilesRule_CheckFile_SkipsOversizedFile verifies that a file larger than
// maxCheckableFileSize is skipped instead of being read into memory and echoed
// into the log — even though it is full of Cyrillic and would otherwise be
// reported.
func TestFilesRule_CheckFile_SkipsOversizedFile(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	testFile := filepath.Join(tempDir, "huge.yaml")

	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	// One Cyrillic line, then padding to push the file past the size limit.
	if _, err := f.WriteString("greeting: Привет\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := f.Truncate(maxCheckableFileSize + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	rule := NewFilesRule(nil, nil)
	errorList := &errors.LintRuleErrorsList{}

	rule.CheckFile(mockModule, testFile, errorList)

	if errs := errorList.GetErrors(); len(errs) > 0 {
		t.Errorf("expected oversized file to be skipped, got %d error(s)", len(errs))
	}
}

// TestFilesRule_CheckFile_CapsReportedLines verifies that a file with more
// Cyrillic lines than maxCyrillicReportLines produces a bounded finding with a
// truncation note rather than echoing every line.
func TestFilesRule_CheckFile_CapsReportedLines(t *testing.T) {
	mc := minimock.NewController(t)
	mockModule := mocks.NewModuleMock(mc)

	tempDir := t.TempDir()
	mockModule.GetPathMock.Return(tempDir)

	var sb strings.Builder
	for i := range maxCyrillicReportLines * 3 {
		fmt.Fprintf(&sb, "line%d: Привет\n", i)
	}

	testFile := filepath.Join(tempDir, "many.yaml")
	if err := os.WriteFile(testFile, []byte(sb.String()), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	rule := NewFilesRule(nil, nil)
	errorList := &errors.LintRuleErrorsList{}

	rule.CheckFile(mockModule, testFile, errorList)

	errs := errorList.GetErrors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one finding, got %d", len(errs))
	}

	value := fmt.Sprint(errs[0].ObjectValue)

	if !strings.Contains(value, "truncated") {
		t.Errorf("expected the reported value to be truncated, got:\n%s", value)
	}

	// At most maxCyrillicReportLines offending lines are echoed (each rendered as
	// two lines: the source line and the cursor), plus the truncation note.
	if got := strings.Count(value, "Привет"); got > maxCyrillicReportLines {
		t.Errorf("expected at most %d echoed lines, got %d", maxCyrillicReportLines, got)
	}
}

// TestCheckCyrillicLettersInString_CapsLineWidth verifies that a single very
// long Cyrillic line is truncated in the reported message.
func TestCheckCyrillicLettersInString_CapsLineWidth(t *testing.T) {
	long := strings.Repeat("Ы", maxCyrillicLineWidth*4)

	msg, has := checkCyrillicLettersInString(long)
	if !has {
		t.Fatal("expected Cyrillic to be detected")
	}

	if !strings.Contains(msg, "…") {
		t.Errorf("expected a truncated long line to be marked with an ellipsis, got:\n%s", msg)
	}

	// The reported line (first line of msg) must be bounded to the configured
	// width plus the ellipsis, not the full 4x-width input.
	firstLine, _, _ := strings.Cut(msg, "\n")
	if runes := []rune(firstLine); len(runes) > maxCyrillicLineWidth+1 {
		t.Errorf("expected reported line width <= %d, got %d", maxCyrillicLineWidth+1, len(runes))
	}
}
