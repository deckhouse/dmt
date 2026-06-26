# Hooks Linter

## Overview

The **Hooks Linter** validates module hooks implementation to ensure proper handling of Kubernetes resources and infrastructure components. This linter checks that modules with specific resource types (like Ingress) include the necessary hooks to handle custom certificates and other required functionality.

Hooks are Go or Python scripts that react to Kubernetes resource changes and implement custom logic for module operations. The linter ensures that required hooks are present when modules use specific resource types.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [ingress](#ingress) | Validates copy_custom_certificate hook presence for Ingress resources | ✅ | enabled |
| [tls-certificate](#tls-certificate) | Detects invalid self-signed certificate generation in Go hooks | ✅ | enabled |

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

### tls-certificate

**Purpose:** Detects places in Go hooks that generate invalid self-signed TLS certificates using the `go_lib/hooks/tls_certificate` helpers (`RegisterInternalTLSHook` / `GenerateSelfSignedCert`).

**Description:**

Self-signed certificates produced by these helpers had defects that caused validation failures outside of Go/kube-apiserver (`openssl verify`, Trivy, Java keystores, MaxPatrol). This rule statically scans Go hook source for the known causes, based on [deckhouse/deckhouse#20138](https://github.com/deckhouse/deckhouse/pull/20138).

**What it checks:**

The rule only inspects `.go` files in the module's `hooks/` directory that import:

```go
"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
```

For those files it reports:

1. **Bogus `"requestheader-client"` usage.** This string does not exist in cfssl's KeyUsage/ExtKeyUsage maps, so cfssl silently discards it and emits a certificate with an empty `ExtendedKeyUsage` extension. Strict validators require at least `serverAuth`. Use `"server auth"` instead.
2. **`WithGroups` on a leaf certificate.** Passing `WithGroups(...)` into a `GenerateSelfSignedCert(...)` call copies the CA's `Organization` onto the leaf, recreating the `Subject == Issuer` (depth-0 self-signed) collision that OpenSSL rejects with `X509_V_ERR_DEPTH_ZERO_SELF_SIGNED_CERT`.

**Why it matters:**

Certificates that pass Go's lenient verification still fail in OpenSSL, Trivy, and other strict validators, breaking webhooks and security scans in environments outside of kube-apiserver.

**Examples:**

❌ **Incorrect** - bogus usage that drops the EKU extension:

```go
import "github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
    Usages: []string{"requestheader-client"}, // no EKU is emitted
    // ...
})
```

❌ **Incorrect** - `WithGroups` on the leaf recreates Subject == Issuer:

```go
cert, _ := tls_certificate.GenerateSelfSignedCert(
    "leaf",
    ca,
    tls_certificate.WithGroups("Deckhouse"), // copies CA Organization onto the leaf
)
```

✅ **Correct**:

```go
var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
    Usages: []string{"server auth"},
    // ...
})

cert, _ := tls_certificate.GenerateSelfSignedCert("leaf", ca)
```

**Error:**

```
Invalid certificate usage "requestheader-client" produces a certificate with an empty ExtendedKeyUsage extension and is rejected by strict validators. Use "server auth" instead.
```

```
WithGroups applied to a leaf certificate via GenerateSelfSignedCert copies the CA Organization onto the leaf, recreating the Subject == Issuer (depth-0 self-signed) collision rejected by OpenSSL. Remove WithGroups from the leaf certificate.
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  hooks:
    tls-certificate:
      disable: false  # Enable/disable the tls-certificate rule
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
    tls-certificate:
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
