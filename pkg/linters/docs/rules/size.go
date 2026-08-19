// Copyright 2026 Flant JSC
// Licensed under the Apache License, Version 2.0

package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/dmt/pkg"
	"github.com/deckhouse/dmt/pkg/errors"
)

const (
	SizeRuleName = "size"

	// maxDocsSizeBytes is the maximum allowed total size of the documentation
	// files under the docs/ directory, excluding the docs/internal subtree.
	maxDocsSizeBytes = 15 * 1024 * 1024 // 15 MB

	// internalDirName is the docs/ subdirectory that holds content which is not
	// rendered as documentation and is therefore excluded from the size budget.
	internalDirName = "internal"
)

func NewSizeRule() *SizeRule {
	return &SizeRule{
		RuleMeta: pkg.RuleMeta{
			Name: SizeRuleName,
		},
	}
}

type SizeRule struct {
	pkg.RuleMeta
}

// CheckSize sums the size of the documentation files under the docs/ directory
// and reports an error when the total exceeds maxDocsSizeBytes. The docs/internal
// subtree is skipped on purpose: it holds content that is not rendered as
// documentation, so it must not count towards the limit. Every other subfolder is
// scanned recursively. Oversized docs bloat the module artifact and usually
// signal binary assets that don't belong there.
func (r *SizeRule) CheckSize(m pkg.Module, errorList *errors.LintRuleErrorsList) {
	errorList = errorList.WithRule(r.GetName())

	modulePath := m.GetPath()
	if modulePath == "" {
		return
	}

	docsPath := filepath.Join(modulePath, "docs")
	if _, err := os.Stat(docsPath); err != nil {
		return
	}

	internalPath := filepath.Join(docsPath, internalDirName)

	var total int64

	err := filepath.WalkDir(docsPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path == internalPath {
				return filepath.SkipDir
			}

			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		total += info.Size()

		return nil
	})
	if err != nil {
		errorList.
			WithFilePath(docsPath).
			WithValue(err.Error()).
			Error("failed to calculate docs/ directory size")

		return
	}

	if total > maxDocsSizeBytes {
		errorList.
			WithFilePath(docsPath).
			WithValue(formatBytes(total)).
			Errorf("docs/ directory size exceeds the limit of %s", formatBytes(maxDocsSizeBytes))
	}
}

// formatBytes renders a byte count as a human-readable string (e.g. "12.3 MB").
func formatBytes(size int64) string {
	const unit = 1024

	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
