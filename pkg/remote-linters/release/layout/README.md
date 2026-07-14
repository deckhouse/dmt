# Layout Linter (release)

## Overview

The **Layout Linter** validates the top-level file layout of a module's **release** image.

This linter is part of the `dmt` remote-lint bundle used to validate a module image pulled from a registry. 

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [changelog](#changelog) | Validates presence of `changelog.yaml` | ❌ | enabled |
| [module-definition](#module-definition) | Validates presence of `module.yaml` | ❌ | enabled |
| [version-json](#version-json) | Validates presence of `version.json` | ❌ | enabled |

## Rule Details

### changelog

**Purpose:** Ensures the release image segment has a `changelog.yaml` file in its root, giving every release a machine-readable history entry point.

**What it checks:**

1. Verifies that `changelog.yaml` exists in the root of the release segment
2. Verifies that `changelog.yaml` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `changelog.yaml`:

```
release/
└── (no changelog.yaml)
```

**Error:**
```
changelog.yaml file is missing in package root
```

❌ **Incorrect** - `changelog.yaml` is a directory:

```
release/
└── changelog.yaml/        # Wrong: must be a file
    └── v1.0.0.yaml
```

**Error:**
```
changelog.yaml must be a file in package root
```

✅ **Correct:**

```
release/
├── changelog.yaml
├── module.yaml
└── version.json
```

---

### module-definition

**Purpose:** Ensures the release image segment has a `module.yaml` file in its root, so the module definition shipped in a release can always be read without falling back to the main bundle image.

**What it checks:**

1. Verifies that `module.yaml` exists in the root of the release segment
2. Verifies that `module.yaml` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `module.yaml`:

```
release/
└── changelog.yaml
```

**Error:**
```
module.yaml file is missing in package root
```

❌ **Incorrect** - `module.yaml` is a directory:

```
release/
└── module.yaml/            # Wrong: must be a file
    └── module.yaml
```

**Error:**
```
module.yaml must be a file in package root
```

✅ **Correct:**

```
release/
├── changelog.yaml
├── module.yaml
└── version.json
```

---

### version-json

**Purpose:** Ensures the release image segment has a `version.json` file in its root, so the released version can be read directly from the release segment.

**What it checks:**

1. Verifies that `version.json` exists in the root of the release segment
2. Verifies that `version.json` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `version.json`:

```
release/
├── changelog.yaml
└── module.yaml
```

**Error:**
```
version.json file is missing in package root
```

❌ **Incorrect** - `version.json` is a directory:

```
release/
└── version.json/           # Wrong: must be a file
    └── v1.0.0.json
```

**Error:**
```
version.json must be a file in package root
```

✅ **Correct:**

```
release/
├── changelog.yaml
├── module.yaml
└── version.json
```

---

## Common Issues

### Issue: Missing `changelog.yaml`, `module.yaml`, or `version.json` in the release segment

**Symptom:**
```
Error: changelog.yaml file is missing in package root
Error: module.yaml file is missing in package root
Error: version.json file is missing in package root
```

**Cause:** The release image segment doesn't have one or more of the required files in its root.

**Solution:** Add the missing file(s) to whatever build step produces the release image segment.

```bash
cd release
touch changelog.yaml module.yaml version.json
```

### Issue: Required file has the wrong kind (must be a file, not a directory)

**Symptom:**
```
Error: changelog.yaml must be a file in package root
Error: module.yaml must be a file in package root
Error: version.json must be a file in package root
```

**Cause:** One of the required files was created as a directory instead of a regular file.

**Solution:** Remove the directory and replace it with a regular file.

```bash
rm -rf version.json
touch version.json
```
