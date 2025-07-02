#!/bin/bash

# Quick lint check and auto-fix script
# Usage: ./scripts/lint-check.sh [--fix]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "🔍 Running golangci-lint check..."

# Capture exit code from make command
if [[ "$1" == "--fix" ]]; then
    echo "🔧 Auto-fixing issues..."
    make lint-fix-fast
    EXIT_CODE=$?
else
    echo "⚡ Running fast lint check..."
    make lint-fast
    EXIT_CODE=$?
fi

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Lint check passed!"
    exit 0
else
    echo "❌ Lint check failed!"
    echo ""
    echo "To auto-fix issues, run:"
    echo "  ./scripts/lint-check.sh --fix"
    echo ""
    echo "Or manually run:"
    echo "  make lint-fix"
    exit 1
fi
