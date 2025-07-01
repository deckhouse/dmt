#!/bin/bash

# Pre-commit hook for automatic linting
# This script will be called before each commit

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "🔍 Pre-commit: Running lint check..."

# Run fast lint check
if ! make lint-fast >/dev/null 2>&1; then
    echo "❌ Lint check failed! Attempting to auto-fix..."

    # Try to auto-fix issues
    if make lint-fix-fast >/dev/null 2>&1; then
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
