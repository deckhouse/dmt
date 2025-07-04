# Linting Scripts

This directory contains scripts for automatic code checking and fixing using golangci-lint.

## Quick Setup

```bash
# Set up git hooks for automatic pre-commit checks
make setup-hooks
```

## Main Commands

```bash
# Full lint check
make lint

# Fast lint check (only fast linters)
make lint-fast

# Full lint check with auto-fix
make lint-fix

# Fast lint check with auto-fix
make lint-fix-fast
```

## Scripts

```bash
# Fast check
./scripts/lint-check.sh

# Fast check with auto-fix
./scripts/lint-check.sh --fix
```

## Git Hooks

After `make setup-hooks`, a pre-commit hook will:

1. Run a fast lint check before each commit
2. Try to auto-fix issues if found
3. If fixes are applied, show modified files and ask you to commit again
4. If auto-fix fails, prompt you to fix issues manually

## Recommended Workflow

1. Use `make lint-fast` during development for quick checks
2. Pre-commit hook will check code before every commit
3. Use `make lint-fix-fast` to auto-fix issues if needed

## IDE Integration

### VS Code

Add to `.vscode/settings.json`:

```json
{
    "go.lintTool": "golangci-lint",
    "go.lintFlags": ["--fast-only"],
    "go.lintOnSave": "package"
}
```

### GoLand

1. Open Settings → Tools → File Watchers
2. Add a watcher for golangci-lint
3. Command: `golangci-lint run --fast-only --fix`

## Troubleshooting

### Linter not found

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Config issues

Check `.golangci.yml` in the project root. Main options:
- `linters.enable` — enabled linters
- `linters.settings` — linter-specific settings
- `formatters.enable` — enabled formatters (gofmt, goimports)

### Ignoring issues

Use directives in code:

```go
//nolint:gocyclo
func longFunction() {
    // ...
}
```

Or add exceptions in `.golangci.yml`:

```yaml
linters-settings:
  gocyclo:
    min-complexity: 15
``` 