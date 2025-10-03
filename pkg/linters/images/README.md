# Images Linter

A comprehensive linter for validating Docker images, Dockerfiles, werf configurations, and patch management in Deckhouse modules.

## Overview

The Images linter ensures that module container images follow security best practices, use approved base images, maintain proper distroless configurations, and have well-documented patches. It helps maintain consistency, security, and quality across all module images.

## Rules

The Images linter includes **4 validation rules**:

| Rule | Description | Configurable |
|------|-------------|--------------|
| [**dockerfile**](#dockerfile-rule) | Validates Dockerfile base images use approved CI variables | ❌ No |
| [**distroless**](#distroless-rule) | Ensures final images are distroless for security | ✅ Yes |
| [**werf**](#werf-rule) | Validates werf.yaml configuration | ✅ Yes |
| [**patches**](#patches-rule) | Validates patch file structure and documentation | ✅ Yes |

---

## Rule Details

### Dockerfile Rule

Validates that Dockerfiles use approved base image CI variables instead of hardcoded image names.

**Purpose:** Ensures consistent base image usage across all modules by enforcing the use of standardized CI variables. This allows centralized base image management, security updates, and version control without modifying individual Dockerfiles.

**Checks:**
- ✅ All `FROM` instructions use approved `$BASE_*` variables
- ✅ No hardcoded image names (e.g., `alpine:3.18`, `golang:1.20`)

**Approved Base Image Variables:**

| CI Variable | Image Pattern | Use Case |
|-------------|---------------|----------|
| `$BASE_ALPINE` | `alpine:X.Y` | Minimal Alpine Linux base |
| `$BASE_GOLANG_ALPINE` | `golang:1.15.X-alpine3.12` | Go builds (Alpine) |
| `$BASE_GOLANG_16_ALPINE` | `golang:1.16.X-alpine3.12` | Go 1.16 builds (Alpine) |
| `$BASE_GOLANG_BUSTER` | `golang:1.15.X-buster` | Go builds (Debian) |
| `$BASE_GOLANG_16_BUSTER` | `golang:1.16.X-buster` | Go 1.16 builds (Debian) |
| `$BASE_NGINX_ALPINE` | `nginx:X.Y-alpine` | NGINX server |
| `$BASE_PYTHON_ALPINE` | `python:X.Y-alpine` | Python applications |
| `$BASE_UBUNTU` | `ubuntu:X.Y` | Ubuntu base |
| `$BASE_JEKYLL` | `jekyll/jekyll:X.Y` | Jekyll static sites |
| `$BASE_SCRATCH` | `scratch:X.Y` | Empty base (static binaries) |

**Example:**

❌ **Incorrect:**
```dockerfile
# Hardcoded image versions
FROM alpine:3.18
FROM golang:1.20-alpine
```

✅ **Correct:**
```dockerfile
# Use approved CI variables
FROM $BASE_ALPINE
FROM $BASE_GOLANG_16_ALPINE
```

**Exclude Images:**
```yaml
# .dmtlint.yaml
linters-settings:
  images:
    exclude-rules:
      skip-image-file-path-prefix:
        - "updater"        # Skip images/updater/
        - "legacy"         # Skip images/legacy/
```

---

### Distroless Rule

Ensures final Docker images are distroless (minimal, secure images without unnecessary tools).

**Purpose:** Enforces security best practices by ensuring production images are distroless, which significantly reduces attack surface by removing shells, package managers, and other unnecessary utilities. Distroless images contain only the application and its runtime dependencies.

**Checks:**
- ✅ Final `FROM` instruction uses `$BASE_DISTROLESS` or `$BASE_ALT`
- ✅ Intermediate `FROM` instructions use `$BASE_*` variables or SHA256 checksums
- ✅ Special case: `scratch` is allowed for static binaries

**Distroless Validation Rules:**

1. **Final Stage (last `FROM`):**
   - Must use `$BASE_DISTROLESS` or `$BASE_ALT`
   - Exception: `FROM scratch` is allowed

2. **Intermediate Stages:**
   - Must use `$BASE_*` variables, OR
   - Must specify SHA256 checksum: `@sha256:...`

**Approved Distroless Prefixes:**
- `$BASE_DISTROLESS` - Distroless images
- `$BASE_ALT` - ALT Linux minimal images
- `.Images.$BASE_DISTROLESS` - Alternative syntax
- `.Images.$BASE_ALT` - Alternative syntax

**Example:**

❌ **Incorrect:**
```dockerfile
# Intermediate stage without SHA256
FROM node:18-alpine AS builder
RUN npm build

# Final stage not distroless
FROM alpine:3.18
COPY --from=builder /app/dist /app
```

✅ **Correct:**
```dockerfile
# Intermediate stage with SHA256 checksum
FROM node:18-alpine@sha256:a1b2c3d4... AS builder
RUN npm build

# Final stage using distroless
FROM $BASE_DISTROLESS
COPY --from=builder /app/dist /app
```

✅ **Also Correct (using $BASE_ variables):**
```dockerfile
# Intermediate stage with approved variable
FROM $BASE_GOLANG_16_ALPINE AS builder
RUN go build

# Final stage using scratch for static binary
FROM scratch
COPY --from=builder /app/binary /binary
```

**Exclude Images:**
```yaml
# .dmtlint.yaml
linters-settings:
  images:
    exclude-rules:
      skip-distroless-file-path-prefix:
        - "updater"        # Skip distroless check for updater
        - "debug-tools"    # Skip for debug tools
```

**Warning:** Excluded images will show `WARNING!!! SKIP DISTROLESS CHECK!!!` messages but won't fail the lint.

---

### Werf Rule

Validates the `werf.yaml` configuration file for image building.

**Purpose:** Ensures werf.yaml follows best practices for building module images, including proper base image usage, avoidance of deprecated directives, and correct user configuration. This maintains build consistency and prevents security issues from user override.

**Checks:**
- ✅ `fromImage` uses approved base images (`base/*` or `common/*`)
- ✅ No deprecated `artifact:` directive (use `from:` or `fromImage:` with `final: false`)
- ✅ `imageSpec.config.user` is not overridden (except istio, ingress-nginx)
- ✅ Valid YAML structure
- ✅ Images belong to the correct module

**Werf Configuration Structure:**
```yaml
# werf.yaml
configVersion: 1
project: deckhouse
---
image: my-module/my-image
from: common/distroless
fromImage: base/distroless
final: true  # or false for intermediate images
imageSpec:
  config:
    user: ""  # Should be empty (inherited from base)
```

**Validation Rules:**

1. **Base Image (`fromImage`):**
   - Must start with `base/` or `common/`
   - Exception: `terraform-manager` module can use custom bases

2. **Deprecated `artifact:` directive:**
   - ❌ Old: `artifact: my-artifact`
   - ✅ New: `fromImage: base/golang` + `final: false`

3. **User Override:**
   - `imageSpec.config.user` must be empty
   - Exceptions: `istio`, `ingress-nginx` (warning only)

**Example:**

❌ **Incorrect:**
```yaml
# Using deprecated artifact directive
artifact: builder
from: $BASE_GOLANG_ALPINE
---
# Custom user override
image: my-module/app
fromImage: base/distroless
imageSpec:
  config:
    user: "nobody"  # ❌ Should not override user
```

✅ **Correct:**
```yaml
# Intermediate build stage
image: my-module/builder
fromImage: base/golang-alpine
final: false
---
# Final distroless image
image: my-module/app
fromImage: base/distroless
final: true
imageSpec:
  config:
    user: ""  # ✅ Empty, inherited from base
```

**Configuration:**
```yaml
# .dmtlint.yaml
linters-settings:
  images:
    werf:
      disable: false  # Enable werf validation
```

---

### Patches Rule

Validates patch file organization, naming, and documentation.

**Purpose:** Ensures image patches are properly organized, documented, and tracked. Well-documented patches make it easier to understand why modifications were made, track upstream changes, and maintain patches across updates.

**Checks:**
- ✅ Patch files are in `images/<image_name>/patches/` directory
- ✅ Patch file names follow pattern: `XXX-<patch-name>.patch` (e.g., `001-fix-ssl.patch`)
- ✅ Each patches directory has a `README.md` file
- ✅ README.md documents each patch with `# XXX-<patch-name>.patch` header

**Patch Directory Structure:**
```
images/
└── my-image/
    ├── Dockerfile
    └── patches/
        ├── README.md               # Required documentation
        ├── 001-fix-ssl.patch       # Patch files
        ├── 002-update-config.patch
        └── 003-security-fix.patch
```

**Patch Naming Convention:**
- Format: `XXX-<description>.patch`
- `XXX` = Three-digit number (001, 002, 003, ...)
- `<description>` = Descriptive name (lowercase, hyphens)
- Extension: `.patch`

**README.md Format:**
```markdown
# Patches

## 001-fix-ssl.patch
- **Issue**: SSL handshake fails with certain clients
- **Upstream**: https://github.com/project/repo/issues/1234
- **Status**: Waiting for upstream fix in v2.0

## 002-update-config.patch
- **Issue**: Default config incompatible with Kubernetes
- **Upstream**: PR submitted https://github.com/project/repo/pull/5678
- **Status**: Merged, will be in next release

## 003-security-fix.patch
- **Issue**: CVE-2024-12345 vulnerability
- **Upstream**: https://github.com/project/repo/security/advisories/GHSA-xxxx
- **Status**: Backport from upstream fix
```

**Example:**

❌ **Incorrect:**
```
images/
└── nginx/
    ├── Dockerfile
    ├── fix-ssl.patch        # ❌ Not in patches/ directory
    └── patches/
        ├── update.patch     # ❌ No number prefix
        └── 1-fix.patch      # ❌ Only one digit
        # ❌ Missing README.md
```

✅ **Correct:**
```
images/
└── nginx/
    ├── Dockerfile
    └── patches/
        ├── README.md               # ✅ Documentation present
        ├── 001-fix-ssl.patch       # ✅ Proper naming
        ├── 002-update-config.patch # ✅ Sequential numbering
        └── 003-security-fix.patch  # ✅ All documented in README
```

**Configuration:**
```yaml
# .dmtlint.yaml
linters-settings:
  images:
    patches:
      disable: false  # Enable patch validation
```

**Error Messages:**
- `Patch file should be in images/<image_name>/patches/ directory`
- `Patch file should have a corresponding README file`
- `Patch file name should match pattern XXX-<patch-name>.patch`
- `README.md file does not contain # XXX-patch-name.patch`

---

## Configuration

### Module-Level Settings

Configure the Images linter in `.dmtlint.yaml`:

```yaml
linters-settings:
  images:
    # Exclude specific image paths from dockerfile checks
    exclude-rules:
      skip-image-file-path-prefix:
        - "updater"          # Skip images/updater/
        - "legacy"           # Skip images/legacy/
      
      # Exclude specific image paths from distroless checks
      skip-distroless-file-path-prefix:
        - "updater"          # Skip distroless validation
        - "debug-tools"      # Skip for debug images
    
    # Rule-specific settings
    patches:
      disable: false         # Enable/disable patches validation
    
    werf:
      disable: false         # Enable/disable werf validation
    
    # Overall impact level
    impact: error            # Level: error | warning | info
```

### Rule-Level Configuration

Configure individual rule severity:

```yaml
linters-settings:
  images:
    rules:
      dockerfile:
        level: error         # error | warning | info | ignored
      distroless:
        level: error
      werf:
        level: error
      patches:
        level: warning
```

---

## Common Issues

### ❌ Hardcoded Base Image

**Error:** `Please use $BASE_ALPINE as an image name`

**Solution:** Replace hardcoded image with approved CI variable:
```dockerfile
# Before
FROM alpine:3.18

# After
FROM $BASE_ALPINE
```

### ❌ Non-Distroless Final Image

**Error:** `Last FROM instruction should use one of our $BASE_DISTROLESS images`

**Solution:** Use distroless base for final stage:
```dockerfile
# Build stage
FROM $BASE_GOLANG_16_ALPINE AS builder
RUN go build -o /app main.go

# Final stage - use distroless
FROM $BASE_DISTROLESS
COPY --from=builder /app /app
```

### ❌ Intermediate Image Without Checksum

**Error:** `Intermediate FROM instructions should use one of our $BASE_ images or have @sha526: checksum specified`

**Solution:** Either use `$BASE_*` variable or add SHA256 checksum:
```dockerfile
# Option 1: Use $BASE_ variable
FROM $BASE_GOLANG_16_ALPINE AS builder

# Option 2: Add SHA256 checksum
FROM golang:1.20-alpine@sha256:abc123... AS builder
```

### ❌ Deprecated Artifact Directive

**Error:** `Use from: or fromImage: and final: false directives instead of artifact: in the werf file`

**Solution:** Migrate to modern syntax:
```yaml
# Before
artifact: builder
from: $BASE_GOLANG_ALPINE

# After
image: my-module/builder
fromImage: base/golang-alpine
final: false
```

### ❌ Patch File Not Documented

**Error:** `README.md file does not contain # 001-fix-ssl.patch`

**Solution:** Add documentation to `patches/README.md`:
```markdown
# Patches

## 001-fix-ssl.patch
- **Issue**: Description of the issue
- **Upstream**: Link to upstream issue/PR
- **Status**: Current status
```

### ❌ Wrong Patch Directory

**Error:** `Patch file should be in images/<image_name>/patches/ directory`

**Solution:** Move patch to correct directory:
```bash
# Wrong location
images/nginx/001-fix.patch

# Correct location
images/nginx/patches/001-fix.patch
```

### ❌ Invalid Patch Naming

**Error:** `Patch file name should match pattern XXX-<patch-name>.patch`

**Solution:** Rename with three-digit prefix:
```bash
# Before
fix.patch
1-fix.patch

# After
001-fix.patch
```
