#!/bin/bash

# Pre-commit hook for automatic linting
# This script will be called before each commit

set -e

PROJECT_ROOT="$(git rev-parse --show-toplevel)"

cd "$PROJECT_ROOT"

echo "🔍 Pre-commit: Running lint check..."

# Run fast lint check
if ! make -f "$PROJECT_ROOT/Makefile" lint-fast; then
    echo "❌ Lint check failed! Attempting to auto-fix..."

    # Try to auto-fix issues
    if make -f "$PROJECT_ROOT/Makefile" lint-fix-fast; then
        echo "✅ Issues auto-fixed! Please review changes and commit again."
        echo "   Modified files:"
        git diff --name-only --cached
        exit 1
    else
        echo "❌ Auto-fix failed. Please fix issues manually:"
        echo "   make lint-fix"
        exit 1
    fi
fi

echo "✅ Pre-commit check passed!"
exit 0
