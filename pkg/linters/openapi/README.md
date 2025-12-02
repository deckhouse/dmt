# OpenAPI Linter

## Overview

The **OpenAPI Linter** validates OpenAPI schema files and Custom Resource Definitions (CRDs) to ensure compliance with Kubernetes API conventions, Deckhouse standards, and best practices. This linter checks schema files in the `openapi/` and `crds/` directories, validating enum values, special field constraints, deprecated key usage, and CRD metadata.

Proper OpenAPI schema validation is critical for module configuration, ensuring type safety, consistent API design, and compatibility with Kubernetes conventions. The linter helps prevent configuration errors and maintains API consistency across all Deckhouse modules.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [enum](#enum) | Validates enum values follow Kubernetes CamelCase conventions | ✅ | enabled |
| [high-availability](#high-availability) | Validates highAvailability field has no default value | ✅ | enabled |
| [keys](#keys) | Validates property names don't use banned names | ✅ | enabled |
| [deckhouse-crds](#deckhouse-crds) | Validates Deckhouse CRD structure and metadata | ✅ | enabled |

## Rule Details

### enum

**Purpose:** Ensures enum values in OpenAPI schemas follow Kubernetes API conventions for consistency, readability, and compatibility with Kubernetes tooling. Proper enum formatting makes APIs predictable and maintainable.

**Description:**

Validates that all enum values in OpenAPI schema files follow the Kubernetes CamelCase convention. According to [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#constants), enum values must be strings in CamelCase with an initial uppercase letter.

**What it checks:**

1. Scans all `.yaml` and `.yml` files in `openapi/` and `crds/` directories
2. Identifies enum fields in OpenAPI schemas
3. Validates each enum value follows CamelCase convention:
   - Must start with an uppercase letter (if it starts with a letter)
   - Must contain only letters and numbers
   - Must not contain spaces, hyphens, or underscores (except in special cases)
   - Numbers can include dots for float values (e.g., `1.5`)

**Kubernetes API Conventions:**

From the [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#constants):

> Some fields will have a list of allowed values (enumerations). These values will be strings, and they will be in CamelCase, with an initial uppercase letter. Examples: `ClusterFirst`, `Pending`, `ClientIP`.
>
> When an acronym or initialism, each letter in the acronym should be uppercase, such as with `ClientIP` or `TCPDelay`.
>
> When a proper name or the name of a command-line executable is used as a constant, the proper name should be represented in consistent casing - examples: `systemd`, `iptables`, `IPVS`, `cgroupfs`, `Docker` (as a generic concept), `docker` (as the command-line executable). If a proper name is used which has mixed capitalization like `eBPF`, that should be preserved in a longer constant such as `eBPFDelegation`.

**Why it matters:**

1. **Consistency**: Follows Kubernetes API conventions used throughout the ecosystem
2. **Readability**: CamelCase enum values are easier to read and understand
3. **Tooling Compatibility**: Kubernetes tools expect enum values in this format
4. **API Stability**: Consistent naming reduces confusion and API changes

**Examples:**

❌ **Incorrect** - Invalid enum values:

```yaml
# openapi/config-values.yaml
properties:
  logLevel:
    type: string
    enum:
      - debug        # ❌ Must start with uppercase
      - info         # ❌ Must start with uppercase
      - WARNING      # ❌ All caps (unless it's an acronym)
      - error-level  # ❌ Contains hyphen
```

**Error:**
```
Error: enum 'properties.logLevel.enum' is invalid: value 'debug' must start with Capital letter
Error: enum 'properties.logLevel.enum' is invalid: value 'info' must start with Capital letter
Error: enum 'properties.logLevel.enum' is invalid: value 'error-level' must be in CamelCase
```

❌ **Incorrect** - Invalid special characters:

```yaml
properties:
  storageType:
    type: string
    enum:
      - local_storage    # ❌ Contains underscore
      - network-storage  # ❌ Contains hyphen
      - cloud storage    # ❌ Contains space
```

**Error:**
```
Error: enum 'properties.storageType.enum' is invalid: value 'local_storage' must be in CamelCase
Error: enum 'properties.storageType.enum' is invalid: value 'network-storage' must be in CamelCase
Error: enum 'properties.storageType.enum' is invalid: value 'cloud storage' must be in CamelCase
```

✅ **Correct** - Valid enum values:

```yaml
# openapi/config-values.yaml
properties:
  logLevel:
    type: string
    enum:
      - Debug
      - Info
      - Warning
      - Error
```

✅ **Correct** - Acronyms and proper names:

```yaml
properties:
  networkPolicy:
    type: string
    enum:
      - ClusterFirst          # Standard CamelCase
      - ClientIP              # Acronym - all caps
      - TCPDelay              # Acronym - all caps
```

✅ **Correct** - Proper names and executables:

```yaml
properties:
  runtime:
    type: string
    enum:
      - Docker                # Proper name (generic concept)
      - docker                # Executable name (command-line)
      - containerd            # Executable name
      - systemd               # Executable name
      - iptables              # Executable name
```

✅ **Correct** - Mixed capitalization proper names:

```yaml
properties:
  ebpfMode:
    type: string
    enum:
      - eBPFDelegation        # Preserves eBPF capitalization
      - eBPFNative
```

✅ **Correct** - Numbers in enum values:

```yaml
properties:
  version:
    type: string
    enum:
      - Version1              # With number
      - Version2
      - TLS1.2                # With dot in number
      - TLS1.3
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    impact: error
```

To exclude specific enum fields:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      enum:
        - "properties.storageClass.properties.type"
        - "properties.legacy.properties.mode"
```

To exclude enum fields with array wildcards:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      enum:
        # Exclude specific array item properties
        - "properties.items[*].properties.type"
        - "properties.provision.items.oneOf[*].properties.mode"
```

---

### high-availability

**Purpose:** Ensures the `highAvailability` field in OpenAPI schemas doesn't have a default value. This field must be explicitly set by users to avoid unintended behavior and resource allocation in high-availability configurations.

**Description:**

Validates that the `properties.highAvailability` field in OpenAPI schemas doesn't define a `default` value. High availability settings should always be explicitly configured by users, never assumed by default.

**What it checks:**

1. Identifies `properties.highAvailability` fields in OpenAPI schemas
2. Validates the field has no `default` key defined
3. Ensures users must explicitly enable or disable high availability

**Why it matters:**

1. **Explicit Configuration**: High availability requires deliberate choice
2. **Resource Impact**: HA configurations consume more resources
3. **Production Safety**: Prevents accidental HA enablement in development
4. **Cost Awareness**: Users should consciously decide on HA for cost implications

**Examples:**

❌ **Incorrect** - Has default value:

```yaml
# openapi/config-values.yaml
properties:
  highAvailability:
    type: boolean
    default: true    # ❌ Must not have default value
    description: Enable high availability mode
```

**Error:**
```
Error: properties.highAvailability is invalid: must have no default value
```

❌ **Incorrect** - Default in complex schema:

```yaml
properties:
  highAvailability:
    type: object
    default: {}      # ❌ Must not have default value
    properties:
      enabled:
        type: boolean
```

**Error:**
```
Error: properties.highAvailability is invalid: must have no default value
```

✅ **Correct** - No default value:

```yaml
# openapi/config-values.yaml
properties:
  highAvailability:
    type: boolean
    description: |
      Enable high availability mode.
      When enabled, runs multiple replicas with anti-affinity.
```

✅ **Correct** - Complex schema without default:

```yaml
properties:
  highAvailability:
    type: object
    description: High availability configuration
    properties:
      enabled:
        type: boolean
        description: Enable high availability
      replicas:
        type: integer
        minimum: 2
        description: Number of replicas in HA mode
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      ha-absolute-keys:
        - "properties.highAvailability"  # Exclude specific HA field
```

---

### keys

**Purpose:** Prevents use of banned names in enum values within OpenAPI schemas. This ensures enum values don't conflict with reserved keywords or cause confusion.

**Description:**

Validates that enum values in OpenAPI schema files (specifically in CRD files in `crds/` directory) don't use banned keywords. Banned names are typically reserved words that could cause conflicts with Kubernetes or Deckhouse internals.

**What it checks:**

1. Scans enum fields in CRD OpenAPI schemas
2. Checks each enum value against the banned names list
3. Recursively validates nested structures
4. Reports any usage of banned keywords in enum values

**Why it matters:**

1. **Conflict Prevention**: Avoids conflicts with reserved keywords
2. **API Clarity**: Prevents confusing or ambiguous property names
3. **Future Compatibility**: Reserved names may be used in future versions
4. **Standard Compliance**: Maintains naming convention standards

**Examples:**

❌ **Incorrect** - Using banned name in enum:

```yaml
# Assume "default" is a banned name
properties:
  mode:
    type: string
    enum:
      - Standard
      - default      # ❌ Banned name
      - Custom
```

**Error:**
```
Error: default is invalid name for property default
```

❌ **Incorrect** - Banned name in property:

```yaml
properties:
  settings:
    type: object
    properties:
      default:       # ❌ Banned property name
        type: string
```

**Error:**
```
Error: validation error: wrong property: default is invalid name for property default
```

✅ **Correct** - Valid property names:

```yaml
properties:
  mode:
    type: string
    enum:
      - Standard
      - Custom
      - Advanced
  
  settings:
    type: object
    properties:
      preset:        # Not banned
        type: string
      configuration:
        type: string
```

**Configuration:**

Define which names should be banned in enum values:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      key-banned-names:
        - "default"    # Ban "default" as an enum value
        - "type"       # Ban "type" as an enum value
```

**Note:** The `key-banned-names` list defines which names are **not allowed** as enum values. Common banned names typically include:
- `default` - Reserved for schema defaults
- `type` - Reserved for OpenAPI type definitions
- Other context-specific reserved words

---

### deckhouse-crds

**Purpose:** Ensures Custom Resource Definitions (CRDs) for Deckhouse follow proper structure, use current API versions, have required labels, and don't use deprecated fields. This maintains CRD quality and compatibility with Deckhouse standards.

**Description:**

Validates CRDs in the `crds/` directory that belong to Deckhouse (contain `deckhouse.io` in the name). Checks API version, required labels, and validates against use of deprecated `deprecated` key in favor of the Deckhouse-specific `x-doc-deprecated` annotation.

**What it checks:**

1. CRD API version is `apiextensions.k8s.io/v1` (not deprecated versions)
2. CRD has required `module` label matching the module name
3. CRD doesn't use the deprecated `deprecated` key in properties
4. Validates that `x-doc-deprecated: true` is used instead of `deprecated`

**Why it matters:**

1. **API Compatibility**: Current API versions ensure Kubernetes compatibility
2. **Module Tracking**: Labels enable proper resource management
3. **Documentation Standards**: Deckhouse uses custom deprecation annotations
4. **Future-Proofing**: Prevents use of deprecated Kubernetes features

**Examples:**

❌ **Incorrect** - Deprecated API version:

```yaml
# crds/my-resource.yaml
apiVersion: apiextensions.k8s.io/v1beta1  # ❌ Deprecated
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: MyResource
```

**Error:**
```
Error: CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"
```

❌ **Incorrect** - Missing module label:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
  labels:
    # ❌ Missing: module label
    app: my-app
```

**Error:**
```
Error: CRD should contain "module = my-module" label
```

❌ **Incorrect** - Wrong module label value:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
  labels:
    module: wrong-name  # ❌ Should match actual module name
```

**Error:**
```
Error: CRD should contain "module = my-module" label, but got "module = wrong-name"
```

❌ **Incorrect** - Using deprecated key:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
spec:
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                oldField:
                  type: string
                  deprecated: true  # ❌ Use x-doc-deprecated instead
```

**Error:**
```
Error: CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.spec.properties.oldField", use "x-doc-deprecated: true" instead
```

✅ **Correct** - Proper CRD structure:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
  labels:
    module: my-module
    heritage: deckhouse
spec:
  group: deckhouse.io
  scope: Cluster
  names:
    plural: myresources
    singular: myresource
    kind: MyResource
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                enabled:
                  type: boolean
                  description: Enable the resource
```

✅ **Correct** - Using Deckhouse deprecation:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
  labels:
    module: my-module
spec:
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                oldField:
                  type: string
                  x-doc-deprecated: true  # ✅ Correct Deckhouse deprecation
                  description: |
                    DEPRECATED: Use newField instead.
                    This field will be removed in v2.
                newField:
                  type: string
                  description: Replacement for oldField
```

✅ **Correct** - Multiple versions:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: myresources.deckhouse.io
  labels:
    module: my-module
spec:
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                field1:
                  type: string
    - name: v1alpha1
      served: true
      storage: false
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                legacyField:
                  type: string
                  x-doc-deprecated: true
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      crd-names:
        - projects.deckhouse.io        # Exclude specific CRD
        - projecttemplates.deckhouse.io
        - legacy-resources.deckhouse.io
```

**Note:** Only CRDs with `deckhouse.io` in their name are validated by this rule. Third-party CRDs are automatically skipped.

## Configuration

The OpenAPI linter can be configured at the module level with rule-specific exclusions.

### Module-Level Settings

Configure the overall impact level for the openapi linter:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### Rule-Level Exclusions

Each rule supports excluding specific schema paths or CRD names:

#### Enum Rule Exclusions

Exclude specific enum fields by their schema path:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      enum:
        # Exact path to enum field
        - "properties.storageClass.properties.type"
        
        # Path with array index
        - "properties.items[0].properties.mode"
        
        # Path with wildcard for any array index
        - "properties.provision.items[*].properties.type"
        - "properties.config.oneOf[*].properties.format"
```

#### High Availability Exclusions

Exclude specific highAvailability fields:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      ha-absolute-keys:
        - "properties.highAvailability"
        - "properties.internal.properties.highAvailability"
```

#### Key Banned Names Configuration

Define which property names are banned in enum values:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      key-banned-names:
        - "default"    # Ban "default" as an enum value
        - "type"       # Ban "type" as an enum value
        - "name"       # Ban "name" as an enum value
```

**Note:** This list defines which names are **not allowed** in enum values, not paths to exclude from validation.

#### CRD Exclusions

Exclude specific CRDs from validation:

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    exclude-rules:
      crd-names:
        - projects.deckhouse.io
        - projecttemplates.deckhouse.io
        - legacy-resources.deckhouse.io
```

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  openapi:
    # Global impact level
    impact: error
    
    # Rule-specific exclusions
    exclude-rules:
      # Enum value exclusions (paths to exclude from enum validation)
      enum:
        - "properties.storageClass.properties.provision.items.properties.type"
        - "properties.storageClass.properties.provision.items.oneOf[*].properties.type"
        - "properties.legacy.properties.mode"
      
      # High availability field exclusions
      ha-absolute-keys:
        - "properties.internal.properties.highAvailability"
      
      # Banned names in enum values (names that are not allowed)
      key-banned-names:
        - "default"
        - "type"
      
      # CRD name exclusions
      crd-names:
        - projects.deckhouse.io
        - projecttemplates.deckhouse.io
        - experimental-resources.deckhouse.io
```

### Configuration in Module Directory

You can also place a `.dmt.yaml` configuration file directly in your module directory:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  openapi:
    impact: warning  # More lenient for this specific module
    exclude-rules:
      enum:
        - "properties.legacy.properties.type"
      crd-names:
        - legacy-resource.deckhouse.io
```

## Common Issues

### Issue: Invalid enum case

**Symptom:**
```
Error: enum 'properties.logLevel.enum' is invalid: value 'debug' must start with Capital letter
```

**Cause:** Enum values don't follow Kubernetes CamelCase convention.

**Solutions:**

1. **Capitalize enum values:**

   ```yaml
   # Before
   enum:
     - debug
     - info
     - warning
   
   # After
   enum:
     - Debug
     - Info
     - Warning
   ```

2. **Use proper CamelCase for multi-word values:**

   ```yaml
   # Before
   enum:
     - cluster_first
     - dns_default
   
   # After
   enum:
     - ClusterFirst
     - DNSDefault
   ```

### Issue: Enum with special characters

**Symptom:**
```
Error: enum 'properties.mode.enum' is invalid: value 'local-storage' must be in CamelCase
```

**Cause:** Enum values contain hyphens, underscores, or spaces.

**Solutions:**

1. **Remove special characters and use CamelCase:**

   ```yaml
   # Before
   enum:
     - local-storage
     - network_storage
     - cloud storage
   
   # After
   enum:
     - LocalStorage
     - NetworkStorage
     - CloudStorage
   ```

2. **For proper names, follow Kubernetes conventions:**

   ```yaml
   # Before
   enum:
     - ip-tables
     - e-bpf
   
   # After
   enum:
     - iptables    # Executable name
     - eBPF        # Proper name with mixed case
   ```

### Issue: HighAvailability has default value

**Symptom:**
```
Error: properties.highAvailability is invalid: must have no default value
```

**Cause:** The `highAvailability` field has a `default` key defined.

**Solutions:**

1. **Remove the default value:**

   ```yaml
   # Before
   properties:
     highAvailability:
       type: boolean
       default: true
   
   # After
   properties:
     highAvailability:
       type: boolean
       description: Enable high availability mode
   ```

2. **Document the required explicit configuration:**

   ```yaml
   properties:
     highAvailability:
       type: boolean
       description: |
         Enable high availability mode.
         This must be explicitly set - there is no default value.
         HA mode increases resource usage and should be consciously enabled.
   ```

### Issue: Deprecated API version in CRD

**Symptom:**
```
Error: CRD specified using deprecated api version, wanted "apiextensions.k8s.io/v1"
```

**Cause:** CRD uses `apiextensions.k8s.io/v1beta1` instead of `v1`.

**Solutions:**

1. **Update to current API version:**

   ```yaml
   # Before
   apiVersion: apiextensions.k8s.io/v1beta1
   
   # After
   apiVersion: apiextensions.k8s.io/v1
   ```

2. **Check Kubernetes migration guide:**
   - Review [CRD v1beta1 to v1 migration](https://kubernetes.io/docs/reference/using-api/deprecation-guide/#customresourcedefinition-v122)
   - Update schema structure if needed (v1 has stricter requirements)

### Issue: Missing module label in CRD

**Symptom:**
```
Error: CRD should contain "module = my-module" label
```

**Cause:** CRD is missing the required `module` label.

**Solutions:**

1. **Add the module label:**

   ```yaml
   apiVersion: apiextensions.k8s.io/v1
   kind: CustomResourceDefinition
   metadata:
     name: myresources.deckhouse.io
     labels:
       module: my-module        # Add this label
       heritage: deckhouse
   ```

2. **Use Helm templates to ensure consistency:**

   ```yaml
   # crds/myresource.yaml
   apiVersion: apiextensions.k8s.io/v1
   kind: CustomResourceDefinition
   metadata:
     name: myresources.deckhouse.io
     labels:
       module: {{ .Chart.Name }}
       heritage: deckhouse
   ```

### Issue: Using deprecated key instead of x-doc-deprecated

**Symptom:**
```
Error: CRD contains "deprecated" key at path "spec.versions[].schema.openAPIV3Schema.properties.spec.properties.oldField", use "x-doc-deprecated: true" instead
```

**Cause:** CRD uses Kubernetes `deprecated` key instead of Deckhouse `x-doc-deprecated`.

**Solutions:**

1. **Replace with x-doc-deprecated:**

   ```yaml
   # Before
   properties:
     oldField:
       type: string
       deprecated: true
   
   # After
   properties:
     oldField:
       type: string
       x-doc-deprecated: true
       description: |
         DEPRECATED: Use newField instead.
         This field will be removed in v2.
   ```

2. **Add comprehensive deprecation documentation:**

   ```yaml
   properties:
     oldField:
       type: string
       x-doc-deprecated: true
       x-doc-deprecated-version: "v1.5.0"
       description: |
         DEPRECATED since v1.5.0: Use newField instead.
         
         This field is maintained for backward compatibility and will be
         removed in v2.0.0. Please migrate to using newField.
         
         Migration guide: newField accepts the same values as oldField.
     newField:
       type: string
       description: Replacement for the deprecated oldField
   ```

### Issue: Enum validation in complex nested structures

**Symptom:**
```
Error: enum 'properties.items[5].properties.type.enum' is invalid: value 'custom_type' must be in CamelCase
```

**Cause:** Enum in deeply nested structure has invalid value.

**Solutions:**

1. **Fix the enum value:**

   ```yaml
   # Before
   properties:
     items:
       type: array
       items:
         properties:
           type:
             enum:
               - custom_type
   
   # After
   properties:
     items:
       type: array
       items:
         properties:
           type:
             enum:
               - CustomType
   ```

2. **Exclude specific path if needed:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     openapi:
       exclude-rules:
         enum:
           # Use wildcard for array indices
           - "properties.items[*].properties.type"
   ```
