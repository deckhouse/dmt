# Layout Linter

## Overview

The **Layout Linter** validates the top-level file and directory layout of a module (bundle) image. It checks that a set of required root-level entries — `.gitignore`, `changelog.yaml`, `docs/`, `templates/`, `charts/`, `images_digests.json`, `Chart.yaml`, `version.json`, and `module.yaml` — are present and are of the expected kind (file vs. directory).

Enforcing a consistent root layout makes modules predictable to browse, package, and automate against: tooling that expects these entries to exist doesn't have to special-case modules that are missing them.

This linter is part of the `dmt` remote-lint bundle used to validate a module image pulled from a registry (see `dmt remote-lint`). It runs standalone and does not read `.dmtlint.yaml`, so none of its rules are configurable or excludable.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [gitignore](#gitignore) | Validates presence of `.gitignore` in the package root | ❌ | enabled |
| [changelog](#changelog) | Validates presence of `changelog.yaml` in the package root | ❌ | enabled |
| [docs](#docs) | Validates presence of `docs/` directory in the package root | ❌ | enabled |
| [templates](#templates) | Validates presence of `templates/` directory in the package root | ❌ | enabled |
| [charts](#charts) | Validates presence of `charts/` directory in the package root | ❌ | enabled |
| [digests](#digests) | Validates presence of `images_digests.json` in the package root | ❌ | enabled |
| [chart-yaml](#chart-yaml) | Validates presence of `Chart.yaml` in the package root | ❌ | enabled |
| [version-json](#version-json) | Validates presence of `version.json` in the package root | ❌ | enabled |
| [module-definition](#module-definition) | Validates presence of `module.yaml` in the package root | ❌ | enabled |

None of these rules can be configured or disabled: this linter has no settings beyond the module path, and there is no `.dmtlint.yaml` support for it.

All rules run unconditionally and independently of each other, so a single `Lint` call can report findings for more than one of them at once.

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

### templates

**Purpose:** Ensures every module has a `templates/` directory in its root, so the bundle ships the Helm templates it's meant to render.

**What it checks:**

1. Verifies that `templates/` exists in the package root
2. Verifies that `templates/` is a directory, not a regular file

**Examples:**

❌ **Incorrect** - Missing `templates/` directory:

```
my-module/
├── .gitignore
└── Chart.yaml
```

**Error:**
```
templates directory is missing in package root
```

❌ **Incorrect** - `templates` is a file:

```
my-module/
└── templates              # Wrong: must be a directory
```

**Error:**
```
templates must be a directory in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
└── templates/
    └── deployment.yaml
```

---

### charts

**Purpose:** Ensures every module has a `charts/` directory in its root, so the bundle ships the shared helm-lib with helper templates used by `templates/`.

**What it checks:**

1. Verifies that `charts/` exists in the package root
2. Verifies that `charts/` is a directory, not a regular file

**Examples:**

❌ **Incorrect** - Missing `charts/` directory:

```
my-module/
├── Chart.yaml
└── templates/
    └── deployment.yaml
```

**Error:**
```
charts directory is missing in package root
```

❌ **Incorrect** - `charts` is a file:

```
my-module/
└── charts                 # Wrong: must be a directory
```

**Error:**
```
charts must be a directory in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
├── charts/
│   └── helm_lib/
└── templates/
    └── deployment.yaml
```

---

### digests

**Purpose:** Ensures every module has an `images_digests.json` file in its root, so the bundle carries resolved digests for every image it references.

**What it checks:**

1. Verifies that `images_digests.json` exists in the package root
2. Verifies that `images_digests.json` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `images_digests.json`:

```
my-module/
├── Chart.yaml
└── templates/
    └── deployment.yaml
```

**Error:**
```
images_digests.json file is missing in package root
```

❌ **Incorrect** - `images_digests.json` is a directory:

```
my-module/
└── images_digests.json/   # Wrong: must be a file
    └── digests.json
```

**Error:**
```
images_digests.json must be a file in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
├── images_digests.json
└── templates/
    └── deployment.yaml
```

---

### chart-yaml

**Purpose:** Ensures every module has a `Chart.yaml` file in its root, so the bundle carries a valid Helm chart definition.

**What it checks:**

1. Verifies that `Chart.yaml` exists in the package root
2. Verifies that `Chart.yaml` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `Chart.yaml`:

```
my-module/
└── templates/
    └── deployment.yaml
```

**Error:**
```
Chart.yaml file is missing in package root
```

❌ **Incorrect** - `Chart.yaml` is a directory:

```
my-module/
└── Chart.yaml/             # Wrong: must be a file
    └── Chart.yaml
```

**Error:**
```
Chart.yaml must be a file in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
└── templates/
    └── deployment.yaml
```

---

### version-json

**Purpose:** Ensures every module has a `version.json` file in its root, so the released version can be read directly from the bundle image.

**What it checks:**

1. Verifies that `version.json` exists in the package root
2. Verifies that `version.json` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `version.json`:

```
my-module/
├── Chart.yaml
└── module.yaml
```

**Error:**
```
version.json file is missing in package root
```

❌ **Incorrect** - `version.json` is a directory:

```
my-module/
└── version.json/          # Wrong: must be a file
    └── v1.0.0.json
```

**Error:**
```
version.json must be a file in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
├── module.yaml
└── version.json
```

---

### module-definition

**Purpose:** Ensures every module has a `module.yaml` file in its root, so the module definition is always available directly from the bundle image.

**What it checks:**

1. Verifies that `module.yaml` exists in the package root
2. Verifies that `module.yaml` is a regular file, not a directory

**Examples:**

❌ **Incorrect** - Missing `module.yaml`:

```
my-module/
├── Chart.yaml
└── version.json
```

**Error:**
```
module.yaml file is missing in package root
```

❌ **Incorrect** - `module.yaml` is a directory:

```
my-module/
└── module.yaml/            # Wrong: must be a file
    └── module.yaml
```

**Error:**
```
module.yaml must be a file in package root
```

✅ **Correct:**

```
my-module/
├── Chart.yaml
├── module.yaml
└── version.json
```

---

## Common Issues

### Issue: Missing a required root-level file or directory

**Symptom:**
```
Error: .gitignore file is missing in package root
Error: changelog.yaml file is missing in package root
Error: docs directory is missing in package root
Error: templates directory is missing in package root
Error: charts directory is missing in package root
Error: images_digests.json file is missing in package root
Error: Chart.yaml file is missing in package root
Error: version.json file is missing in package root
Error: module.yaml file is missing in package root
```

**Cause:** The module root is missing one or more of the required entries.

**Solution:**

```bash
cd modules/my-module
touch .gitignore changelog.yaml images_digests.json Chart.yaml version.json module.yaml
mkdir -p docs templates charts
```

### Issue: Required entry has the wrong kind (file vs. directory)

**Symptom:**
```
Error: .gitignore must be a file in package root
Error: docs must be a directory in package root
Error: templates must be a directory in package root
Error: charts must be a directory in package root
```

**Cause:** A path that should be a file is a directory (or vice versa) — e.g. `docs` was created as an empty file instead of a directory.

**Solution:** Remove the wrong entry and recreate it with the expected kind.

```bash
rm docs                    # was created as a file by mistake
mkdir -p docs
```
