# Module Linter

A comprehensive linter for validating Deckhouse modules, ensuring compliance with best practices, standards, and technical requirements.

## Overview

The Module linter performs automated checks on Deckhouse modules to validate configuration files, documentation, licensing, accessibility settings, version requirements, and more. It helps maintain consistency and quality across all modules.

## Rules

The Module linter includes **7 validation rules**:

| Rule | Description | Configurable |
|------|-------------|--------------|
| [**definition-file**](#definition-file-rule) | Validates `module.yaml` structure, accessibility, and update sections | ✅ Yes |
| [**oss**](#oss-rule) | Validates open-source software attribution in `oss.yaml` | ✅ Yes |
| [**conversions**](#conversions-rule) | Validates OpenAPI conversion files and documentation | ✅ Yes |
| [**helmignore**](#helmignore-rule) | Validates `.helmignore` file presence and content | ✅ Yes |
| [**license**](#license-rule) | Validates license headers in source files | ✅ Yes |
| [**requirements**](#requirements-rule) | Validates version requirements for features | ❌ No |
| [**legacy-release-file**](#legacy-release-file-rule) | Checks for deprecated `release.yaml` file | ❌ No |

---

## Rule Details

### Definition File Rule

Validates the `module.yaml` configuration file structure and content.

**Purpose:** Ensures the module's main configuration file is properly structured with valid metadata, accessibility settings for different editions and bundles, and correct version upgrade paths. This prevents deployment issues and maintains consistency across the Deckhouse platform.

**Checks:**
- ✅ File exists and is valid YAML
- ✅ Required fields are present (name, namespace)
- ✅ Optional fields are valid when present
- ✅ Accessibility configuration is correct
- ✅ Update section follows versioning rules

#### Accessibility Validation

The `accessibility` section controls which editions and bundles include the module.

**Valid Editions:**
- `ce` - Community Edition
- `fe` - Free Edition
- `ee` - Enterprise Edition
- `se` - Standard Edition
- `se-plus` - Standard Edition Plus
- `be` - Business Edition
- `_default` - Default behavior override

**Valid Bundles:**
- `Minimal` - Minimal bundle
- `Managed` - Managed bundle
- `Default` - Default bundle

**Validation Rules:**
- `accessibility.editions` is required when `accessibility` is specified
- Each edition must have `available` (boolean) and `enabledInBundles` (array) fields
- `enabledInBundles` must contain only valid bundle names
- Edition names must be from the valid editions list

**Example:**
```yaml
# module.yaml
name: my-module
namespace: d8-my-module
weight: 100
stage: "General Availability"

accessibility:
  editions:
    _default:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
    ee:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
```

#### Update Validation

The `update` section defines version upgrade paths for the module.

**Validation Rules:**
- Both `from` and `to` fields must be populated
- `to` version must be greater than `from` version
- Versions must use `major.minor` format (no patch versions)
- Entries must be sorted: first by `from` version ascending, then by `to` version ascending
- No duplicate `from` versions for the same `to` version

**Example:**
```yaml
# module.yaml
update:
  versions:
    - from: "1.16"
      to: "1.20"
    - from: "1.16"
      to: "1.25"
    - from: "1.17"
      to: "1.20"
    - from: "1.20"
      to: "1.25"
```

---

### OSS Rule

Validates the `oss.yaml` file containing open-source software attribution.

**Purpose:** Ensures proper attribution of open-source software dependencies used in the module. This maintains license compliance, provides transparency about third-party components, and helps users understand the module's dependencies.

**Checks:**
- ✅ File exists in module root
- ✅ Valid YAML structure
- ✅ At least one project is described
- ✅ Each project has required fields:
  - `name` - Project name
  - `url` - Valid project URL
  - `license` - Valid license identifier
  - `version` (optional) - Project version

**Example:**
```yaml
# oss.yaml
- name: nginx
  url: https://nginx.org/
  license: BSD-2-Clause
  version: "1.25.3"

- name: prometheus
  url: https://prometheus.io/
  license: Apache-2.0
  version: "2.48.0"
```

---

### Conversions Rule

Validates OpenAPI conversion files for configuration version upgrades.

**Purpose:** Ensures smooth configuration migrations when module configurations change between versions. Each conversion must have clear, human-readable descriptions in multiple languages so users understand what changes are being applied during upgrades.

**Checks:**
- ✅ Conversion files in `openapi/conversions/` are valid
- ✅ Version files follow naming convention: `v1.yaml`, `v2.yaml`, etc.
- ✅ Each conversion has human-readable descriptions (English and Russian)
- ✅ Version numbers are sequential
- ✅ Descriptions are not empty

**File Structure:**
```
openapi/
├── config-values.yaml          # Current config version
└── conversions/
    ├── v1.yaml                 # Conversion to version 1
    ├── v2.yaml                 # Conversion to version 2
    └── v3.yaml                 # Conversion to version 3
```

**Example:**
```yaml
# openapi/conversions/v2.yaml
version: 2
description:
  en: "Migrated legacy settings to new configuration structure"
  ru: "Перенесены устаревшие настройки в новую структуру конфигурации"
```

---

### Helmignore Rule

Validates the `.helmignore` file in the module root.

**Checks:**
- ✅ File exists in module root
- ✅ Critical Helm files are NOT excluded:
  - `templates/` - Helm template directory
  - `Chart.yaml` - Helm chart metadata

**Purpose:** The `.helmignore` file prevents unnecessary files from being packaged in Helm charts, reducing chart size and improving performance.

**Example:**
```
# .helmignore - Safe patterns
*.md
docs/
tests/
.git/
.github/
hooks/
openapi/

# ❌ NEVER exclude these:
# templates/
# Chart.yaml
```

---

### License Rule

Validates that source files contain proper license headers.

**Purpose:** Ensures all source code files include proper Apache 2.0 license headers, maintaining legal compliance and copyright attribution. This protects the project legally and ensures clarity about code licensing terms.

**Checks:**
- ✅ Go files (`.go`)
- ✅ Shell scripts (`.sh`)
- ✅ Python files (`.py`)
- ✅ Lua files (`.lua`)
- ✅ Executables (files without extension)

**Automatically Skipped:**
- Generated files (`zz_generated*.go`, `pb.go`)
- Documentation files (`docs/`, `.md`)
- Third-party code (`lib/python/`, `charts/helm_lib`)
- Configuration files (`Dockerfile`, `Makefile`)
- Empty files

**Expected License Headers:**

### Apache License 2.0

```go
/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

### Apache License 2.0 Modified

```go
/*
Copyright 2010 SomeAuthor
Copyright 2925 Flant JSC

Modifications made by Flant JSC as part of the Deckhouse project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
```

### Deckhouse Platform Enterprise Edition

```go
/*
Copyright {{YEAR}} Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
```

**Exclude Specific Files:**
```yaml
# .dmtlint.yaml
linters-settings:
  module:
    exclude-rules:
      license:
        files:
          - images/upmeter/stress.sh
          - images/simple-bridge/rootfs/bin/simple-bridge
        directories:
          - hooks/venv/
          - third-party/
```

---

### Requirements Rule

Validates that modules declare minimum version requirements when using advanced features.

**Purpose:** Prevents runtime failures by ensuring modules explicitly declare minimum Deckhouse versions when using features that were introduced in specific versions. This helps users avoid deploying modules to incompatible Deckhouse installations and provides clear upgrade requirements.

**Automatic Detection:**
This rule automatically detects feature usage and ensures proper version constraints are declared in `module.yaml`.

#### Detected Features

| Feature | Min Deckhouse Version | Detection Criteria |
|---------|----------------------|-------------------|
| **Stage Field** | ≥ 1.68.0 | `stage` field present in `module.yaml` |
| **Go Hooks** | ≥ 1.68.0 | `go.mod` with `module-sdk` + `app.Run()` calls |
| **Readiness Probes** | ≥ 1.71.0 | `module-sdk ≥ 0.3` + `app.WithReadiness()` calls |
| **Optional Modules** | ≥ 1.73.0 | `bootstrapped: false` in requirements |

**Validation:**
- ✅ Minimum version constraints are declared
- ✅ Version format is valid (semver)
- ✅ Constraint meets minimum required version
- ✅ Multiple features require compatible versions

**Example:**
```yaml
# module.yaml
name: my-module
namespace: d8-my-module
stage: "General Availability"  # Requires Deckhouse >= 1.68.0

requirements:
  deckhouse: ">= 1.68.0"       # Minimum version declared
  kubernetes: ">= 1.28.0"      # Optional: Kubernetes requirement
  modules:                      # Optional: Module dependencies
    monitoring-ping: ">= 1.0.0"
```

**Supported Constraint Formats:**
- `>= 1.68.0` - Greater than or equal to
- `> 1.68.0` - Greater than
- `= 1.68.0` - Exact version
- `>= 1.68.0, < 2.0.0` - Range

**Error Examples:**
```
❌ Module uses stage field but requirements.deckhouse is not specified
❌ requirements.deckhouse version should be >= 1.68.0 (found: >= 1.60.0)
❌ Invalid version constraint: "not-a-version"
```

---

### Legacy Release File Rule

Checks for the deprecated `release.yaml` file.

**Purpose:** Enforces the migration from the deprecated `release.yaml` format to the modern `version.json` format. This ensures modules use the current versioning standard and prevents confusion from having multiple version sources.

**Validation:**
- ✅ `release.yaml` should NOT exist
- ✅ Version info should be in `version.json` instead

**Migration:**
```bash
# Old (deprecated)
release.yaml

# New (required)
version.json
```

---

## Configuration

### Module-Level Settings

Configure the Module linter in `.dmtlint.yaml`:

```yaml
linters-settings:
  module:
    # Individual rule settings
    definition-file:
      disable: false              # Enable/disable definition file validation
    
    oss:
      disable: false              # Enable/disable OSS validation
    
    conversions:
      disable: false              # Enable/disable conversions validation
    
    helmignore:
      disable: false              # Enable/disable .helmignore validation
    
    # License exclusions
    exclude-rules:
      license:
        files:                    # Exclude specific files
          - images/upmeter/stress.sh
          - images/simple-bridge/rootfs/bin/simple-bridge
        directories:              # Exclude entire directories
          - hooks/venv/
          - third-party/
          - vendor/
    
    # Overall impact level
    impact: error                 # Level: error | warning | info
```

### Rule-Level Configuration

Configure individual rule severity:

```yaml
linters-settings:
  module:
    rules:
      definition-file:
        level: error              # error | warning | info | ignored
      oss:
        level: error
      conversions:
        level: warning
      helmignore:
        level: warning
      license:
        level: error
      requirements:
        level: error
      legacy-release-file:
        level: warning
```

---

## Common Issues

### ❌ Missing module.yaml

**Error:** `Module should have module.yaml`

**Solution:**
```bash
# Create module.yaml with required fields
cat > module.yaml << EOF
name: my-module
namespace: d8-my-module
weight: 100
EOF
```

### ❌ Invalid Accessibility Configuration

**Error:** `Accessibility edition 'xe' is not valid`

**Solution:** Use only valid edition names:
```yaml
accessibility:
  editions:
    ee:  # Valid: ee, ce, fe, se, se-plus, be, _default
      available: true
      enabledInBundles:
        - Default
```

### ❌ Missing License Header

**Error:** `License header not found or invalid`

**Solution:** Add the Apache 2.0 license header to the file, or exclude it:
```yaml
# .dmtlint.yaml
linters-settings:
  module:
    exclude-rules:
      license:
        files:
          - path/to/third-party-file.sh
```

### ❌ Version Requirement Not Met

**Error:** `Module uses stage field but requirements.deckhouse is not specified`

**Solution:** Add minimum version requirement:
```yaml
# module.yaml
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0"
```

### ❌ Update Versions Not Sorted

**Error:** `Update versions must be sorted`

**Solution:** Sort by `from` version, then by `to` version:
```yaml
# ✅ Correct order
update:
  versions:
    - from: "1.16"
      to: "1.20"
    - from: "1.16"
      to: "1.25"
    - from: "1.17"
      to: "1.20"
```
