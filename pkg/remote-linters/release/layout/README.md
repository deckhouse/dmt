# Layout Linter (release)

## Overview

The **Layout Linter** validates the top-level file layout of a module's **release** image.

This linter is part of the `dmt` remote-lint bundle used to validate a module image pulled from a registry. 

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [changelog](#changelog) | Validates presence of `changelog.yaml` | ❌ | enabled |


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
└── changelog.yaml
```

---

## Common Issues

### Issue: Missing `changelog.yaml` in the release segment

**Symptom:**
```
Error: changelog.yaml file is missing in package root
```

**Cause:** The release image segment doesn't have a `changelog.yaml` file in its root.

**Solution:** Add a `changelog.yaml` to whatever build step produces the release image segment.

### Issue: `changelog.yaml` has the wrong kind

**Symptom:**
```
Error: changelog.yaml must be a file in package root
```

**Cause:** `changelog.yaml` was created as a directory instead of a file.

**Solution:** Remove the directory and replace it with a regular `changelog.yaml` file.

```bash
rm -rf changelog.yaml
touch changelog.yaml
```
