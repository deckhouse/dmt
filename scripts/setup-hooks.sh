#!/bin/bash

# Setup git hooks for automatic linting
# This script will install pre-commit hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "🔧 Setting up git hooks..."

# Create .git/hooks directory if it doesn't exist
mkdir -p .git/hooks

# Install pre-commit hook
if [ -f .git/hooks/pre-commit ]; then
    echo "⚠️  Pre-commit hook already exists. Backing up..."
    mv .git/hooks/pre-commit .git/hooks/pre-commit.backup
fi

# Create symlink to our pre-commit script
ln -sf ../../scripts/pre-commit.sh .git/hooks/pre-commit

echo "✅ Git hooks installed successfully!"
echo ""
echo "Available commands:"
echo "  make lint-fast      - Quick lint check"
echo "  make lint-fix-fast  - Quick lint check with auto-fix"
echo "  ./scripts/lint-check.sh --fix  - Run lint check with auto-fix"
echo ""
echo "The pre-commit hook will now run automatically before each commit."
