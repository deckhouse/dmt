# Hooks Linter

## Overview

The **Hooks Linter** validates module hooks implementation to ensure proper handling of Kubernetes resources and infrastructure components. This linter checks that modules with specific resource types (like Ingress) include the necessary hooks to handle custom certificates and other required functionality.

Hooks are Go or Python scripts that react to Kubernetes resource changes and implement custom logic for module operations. The linter ensures that required hooks are present when modules use specific resource types.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [ingress](#ingress) | Validates copy_custom_certificate hook presence for Ingress resources | ✅ | enabled |

## Rule Details

### ingress

**Purpose:** Ensures that modules containing Ingress resources implement the `copy_custom_certificate` hook to properly handle TLS certificates.

**Description:**

When a module defines Ingress resources with TLS configuration, the module must include a hook to copy custom certificates. This rule verifies that modules with Ingress resources have either:
- A dedicated `copy_custom_certificate.go` or `copy_custom_certificate.py` hook file
- Or import and use the `github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate` package in their Go hooks

**What it checks:**

1. Scans module storage for Ingress Kubernetes resources
2. If Ingress resources are found, checks the module's `hooks/` directory for:
   - Direct hook files: `copy_custom_certificate.go` or `copy_custom_certificate.py`
   - Or Go hooks that import the copy_custom_certificate library

**Why it matters:**

Ingress resources with custom TLS certificates require special handling to ensure certificates are properly propagated to the ingress controller. Without the copy_custom_certificate hook, custom certificates won't be processed correctly, leading to TLS configuration failures.

**Examples:**

❌ **Incorrect** - Module with Ingress but no certificate hook:

```
my-module/
├── openapi/
│   └── config-values.yaml
├── templates/
│   └── ingress.yaml          # Contains Ingress resource
└── hooks/
    └── other-hook.go          # No copy_custom_certificate implementation
```

**Error:**
```
Ingress resource exists but module does not have copy_custom_certificate hook
```

✅ **Correct** - Dedicated copy_custom_certificate hook:

```
my-module/
├── openapi/
│   └── config-values.yaml
├── templates/
│   └── ingress.yaml
└── hooks/
    ├── copy_custom_certificate.go    # Dedicated hook file
    └── other-hook.go
```

✅ **Correct** - Using the library import:

```go
// hooks/setup.go
package hooks

import (
    "github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"
    // ... other imports
)

func init() {
    // Register the copy_custom_certificate hook
    copy_custom_certificate.RegisterHook()
}
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    ingress:
      disable: false  # Enable/disable the ingress rule
```

To disable this rule:

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    ingress:
      disable: true
```

## Configuration

The Hooks linter can be configured at both the module level and for individual rules.

### Module-Level Settings

Configure the overall impact level for the hooks linter:

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### Rule-Level Settings

Each rule can be individually configured:

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    impact: error
    ingress:
      disable: false  # true to disable this specific rule
```

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    # Global impact level for all hooks rules
    impact: error
    
    # Rule-specific settings
    ingress:
      disable: false
```

### Configuration in Module Directory

You can also place a `.dmt.yaml` configuration file directly in your module directory for module-specific settings:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  hooks:
    impact: warning  # More lenient for this specific module
    ingress:
      disable: false
```

## Common Issues

### Issue: False positive when using alternative certificate management

**Symptom:**
```
Error: Ingress resource exists but module does not have copy_custom_certificate hook
```

**Cause:** Your module has Ingress resources but uses a different mechanism for certificate management, or the certificates are managed externally.

**Solutions:**

1. **If you need custom certificate support:** Add the copy_custom_certificate hook:

   ```bash
   # Create the hook file
   touch modules/my-module/hooks/copy_custom_certificate.go
   ```

   ```go
   // hooks/copy_custom_certificate.go
   package hooks

   import (
       "github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"
   )

   func init() {
       copy_custom_certificate.RegisterHook()
   }
   ```

2. **If your Ingress doesn't need custom certificates:** Disable the rule for this module:

   ```yaml
   # modules/my-module/.dmt.yaml
   linters-settings:
     hooks:
       ingress:
         disable: true
   ```

### Issue: Hook present but not detected

**Symptom:** You have implemented the copy_custom_certificate hook but the linter still reports an error.

**Cause:** The hook file is not named correctly or the import statement is not in the expected format.

**Solutions:**

1. **Check hook filename:** Ensure the file is named exactly `copy_custom_certificate.go` or `copy_custom_certificate.py`

2. **Verify import statement format:** The import must be exactly:
   ```go
   "github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"
   ```

3. **Check hook location:** The hook file must be in the `hooks/` directory:
   ```
   modules/my-module/hooks/copy_custom_certificate.go
   ```

### Issue: Multiple hook files with the same functionality

**Symptom:** You have both `copy_custom_certificate.go` and other hooks importing the library.

**Cause:** Redundant implementations can cause confusion and maintenance issues.

**Solution:** Choose one approach:

**Option 1** - Dedicated file (recommended for simplicity):
```
hooks/
└── copy_custom_certificate.go
```

**Option 2** - Import in existing hook (recommended when combining with other functionality):
```go
// hooks/ingress_setup.go
import (
    "github.com/deckhouse/deckhouse/go_lib/hooks/copy_custom_certificate"
    // other imports...
)
```
