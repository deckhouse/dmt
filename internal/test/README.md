# Test Command

Run tests against Deckhouse modules. `dmt test` groups module-level testers that validate rendered output and configuration conversions.

## Overview

```bash
dmt test <subcommand> [module-path]
```

`dmt test` discovers every module under the given path (including subdirectories) and runs the selected tester against each one. A tester is *applicable* to a module only when that module ships the inputs the tester needs; modules without those inputs are silently skipped.

Results are printed per module:

- `✅ [<tester>] <module>` — the tester ran and passed.
- `❌ [<tester>] <module>` — the tester ran and reported failures (details follow).

If any test reports a critical error, the command exits with a non-zero status.

## Subcommands

| Subcommand | Purpose | Documentation |
|------------|---------|---------------|
| `conversions` | Validate OpenAPI configuration conversions against declared versions and testcases | [pkg/testers/conversions/README.md](../../pkg/testers/conversions/README.md) |
| `templates` | Render module templates and compare against committed golden snapshots | [pkg/testers/templates/README.md](../../pkg/testers/templates/README.md) |

## Usage

```bash
# Validate conversions for all modules under the current directory
dmt test conversions

# Compare templates against snapshots for a single module
dmt test templates ./modules/my-module

# Refresh snapshots after intentional template changes
dmt test templates ./modules/my-module --update
```
