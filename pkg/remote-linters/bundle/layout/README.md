# Layout Linter

## Overview

The **Layout Linter** validates the top-level file and directory layout of a module (bundle) image. It checks that a small set of required root-level entries — `.gitignore`, `changelog.yaml`, and `docs/` — are present and are of the expected kind (file vs. directory).

Enforcing a consistent root layout makes modules predictable to browse, package, and automate against: tooling that expects `changelog.yaml` or `docs/` to exist doesn't have to special-case modules that are missing them.

This linter is part of the `dmt` remote-lint bundle used to validate a module image pulled from a registry (see `dmt remote-lint`). It runs standalone and does not read `.dmtlint.yaml`, so none of its rules are configurable or excludable.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [gitignore](#gitignore) | Validates presence of `.gitignore` in the package root | ❌ | enabled |
| [changelog](#changelog) | Validates presence of `changelog.yaml` in the package root | ❌ | enabled |
| [docs](#docs) | Validates presence of `docs/` directory in the package root | ❌ | enabled |

None of these rules can be configured or disabled: this linter has no settings beyond the module path, and there is no `.dmtlint.yaml` support for it.

All three rules run unconditionally and independently of each other, so a single `Lint` call can report findings for more than one of them at once.

## Rule Details

### gitignore

**Purpose:** Ensures every module ships a `.gitignore` file in its root, so local artifacts and generated files don't accidentally get committed or bundled.

**What it checks:**

1. Verifies that `.gitignore` exists in the package root
2. Verifies that `.gitignore` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `.gitignore`:

```
my-module/
├── changelog.yaml
├── docs/
│   └── README.md
└── templates/
    └── deployment.yaml
```

**Error:**
```
.gitignore file is missing in package root
```

❌ **Incorrect** - `.gitignore` is a directory:

```
my-module/
└── .gitignore/            # Wrong: must be a file
    └── notes.txt
```

**Error:**
```
.gitignore must be a file in package root
```

✅ **Correct:**

```
my-module/
├── .gitignore
├── changelog.yaml
└── docs/
    └── README.md
```

---

### changelog

**Purpose:** Ensures every module has a `changelog.yaml` file in its root, giving every release a machine-readable history entry point.

**What it checks:**

1. Verifies that `changelog.yaml` exists in the package root
2. Verifies that `changelog.yaml` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `changelog.yaml`:

```
my-module/
├── .gitignore
└── docs/
    └── README.md
```

**Error:**
```
changelog.yaml file is missing in package root
```

❌ **Incorrect** - `changelog.yaml` is a directory:

```
my-module/
└── changelog.yaml/        # Wrong: must be a file
    └── v1.0.0.yaml
```

**Error:**
```
changelog.yaml must be a file in package root
```

✅ **Correct:**

```
my-module/
├── .gitignore
├── changelog.yaml
└── docs/
    └── README.md
```

---

### docs

**Purpose:** Ensures every module has a `docs/` directory in its root, so downstream documentation rules (see the [Documentation Linter](../docs/README.md)) have a directory to inspect.

**What it checks:**

1. Verifies that `docs/` exists in the package root
2. Verifies that `docs/` is a directory, not a regular file

**Examples:**

❌ **Incorrect** - Missing `docs/` directory:

```
my-module/
├── .gitignore
└── changelog.yaml
```

**Error:**
```
docs directory is missing in package root
```

❌ **Incorrect** - `docs` is a file:

```
my-module/
└── docs                   # Wrong: must be a directory
```

**Error:**
```
docs must be a directory in package root
```

✅ **Correct:**

```
my-module/
├── .gitignore
├── changelog.yaml
└── docs/
    └── README.md
```

---

## Common Issues

### Issue: Missing `.gitignore`, `changelog.yaml`, or `docs/`

**Symptom:**
```
Error: .gitignore file is missing in package root
Error: changelog.yaml file is missing in package root
Error: docs directory is missing in package root
```

**Cause:** The module root is missing one or more of the required entries.

**Solution:**

```bash
cd modules/my-module
touch .gitignore
touch changelog.yaml
mkdir -p docs
```

### Issue: Required entry has the wrong kind (file vs. directory)

**Symptom:**
```
Error: .gitignore must be a file in package root
Error: docs must be a directory in package root
```

**Cause:** A path that should be a file is a directory (or vice versa) — e.g. `docs` was created as an empty file instead of a directory.

**Solution:** Remove the wrong entry and recreate it with the expected kind.

```bash
rm docs                    # was created as a file by mistake
mkdir -p docs
```
