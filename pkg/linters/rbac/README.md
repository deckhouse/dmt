# RBAC Linter

## Overview

The **RBAC Linter** validates Role-Based Access Control (RBAC) resources in Deckhouse modules to ensure compliance with security best practices, naming conventions, and organizational standards. This linter checks Roles, ClusterRoles, RoleBindings, ClusterRoleBindings, and ServiceAccounts to enforce consistent RBAC structure and prevent security misconfigurations.

Proper RBAC configuration is critical for Kubernetes security, ensuring least-privilege access, preventing privilege escalation, and maintaining clear separation of concerns. The linter helps prevent common RBAC mistakes and enforces Deckhouse-specific conventions for resource organization and naming.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [user-authz](#user-authz) | Validates user authorization ClusterRoles structure and naming | ❌ | enabled |
| [binding-subject](#binding-subject) | Validates RoleBinding/ClusterRoleBinding subjects reference existing ServiceAccounts | ✅ | enabled |
| [placement](#placement) | Validates RBAC resource placement and naming conventions | ✅ | enabled |
| [wildcards](#wildcards) | Validates Roles/ClusterRoles don't use wildcard permissions | ✅ | enabled |

## Rule Details

### user-authz

**Purpose:** Ensures user authorization ClusterRoles follow Deckhouse naming conventions and are properly structured. This maintains consistency in user access control across all modules and enables the user-authz system to correctly manage user permissions.

**Description:**

Validates that ClusterRoles in the `templates/user-authz-cluster-roles.yaml` file follow strict naming conventions and have required annotations. These ClusterRoles define access levels that can be assigned to users through Deckhouse's user authorization system.

**What it checks:**

1. Only ClusterRole objects are allowed in `templates/user-authz-cluster-roles.yaml`
2. Each ClusterRole has the `user-authz.deckhouse.io/access-level` annotation
3. ClusterRole names follow the pattern: `d8:user-authz:<module-name>:<access-level>`
4. Access level in the name is in kebab-case format

**Why it matters:**

User authorization ClusterRoles are used by Deckhouse to manage user access across modules. Consistent naming and structure enables automated access control, prevents conflicts, and ensures users can be granted appropriate permissions through the user-authz system.

**Examples:**

❌ **Incorrect** - Non-ClusterRole in user-authz file:

```yaml
# templates/user-authz-cluster-roles.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role  # ❌ Must be ClusterRole
metadata:
  name: d8:user-authz:my-module:admin
```

**Error:**
```
Error: Only ClusterRoles can be specified in "templates/user-authz-cluster-roles.yaml"
```

❌ **Incorrect** - Missing access-level annotation:

```yaml
# templates/user-authz-cluster-roles.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:user-authz:my-module:admin
  # ❌ Missing: user-authz.deckhouse.io/access-level annotation
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
```

**Error:**
```
Error: User-authz access ClusterRoles should have annotation "user-authz.deckhouse.io/access-level"
```

❌ **Incorrect** - Wrong naming format:

```yaml
# templates/user-authz-cluster-roles.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-module-admin-role  # ❌ Wrong format
  annotations:
    user-authz.deckhouse.io/access-level: Admin
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
```

**Error:**
```
Error: Name of user-authz ClusterRoles should be "d8:user-authz:my-module:admin"
```

✅ **Correct** - Proper user-authz ClusterRole:

```yaml
# templates/user-authz-cluster-roles.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:user-authz:my-module:admin
  annotations:
    user-authz.deckhouse.io/access-level: Admin
  labels:
    heritage: deckhouse
    module: my-module
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["*"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["*"]
```

✅ **Correct** - Multiple access levels:

```yaml
# templates/user-authz-cluster-roles.yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:user-authz:my-module:admin
  annotations:
    user-authz.deckhouse.io/access-level: Admin
rules:
  - apiGroups: [""]
    resources: ["*"]
    verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:user-authz:my-module:editor
  annotations:
    user-authz.deckhouse.io/access-level: Editor
rules:
  - apiGroups: [""]
    resources: ["pods", "configmaps"]
    verbs: ["get", "list", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:user-authz:my-module:viewer
  annotations:
    user-authz.deckhouse.io/access-level: Viewer
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
```

**Access Level Naming:**

The access level annotation value is converted to kebab-case for the role name:
- `Admin` → `admin`
- `Editor` → `editor`
- `Viewer` → `viewer`
- `CustomAccessLevel` → `custom-access-level`

---

### binding-subject

**Purpose:** Ensures RoleBindings and ClusterRoleBindings reference ServiceAccounts that actually exist in the module. This prevents binding failures and ensures proper access control by validating that all referenced ServiceAccounts are defined.

**Description:**

Validates that ServiceAccount subjects referenced in RoleBindings and ClusterRoleBindings exist in the module's object store. This prevents broken bindings that reference non-existent ServiceAccounts.

**What it checks:**

1. Examines all subjects in RoleBindings and ClusterRoleBindings
2. For ServiceAccount subjects, validates they exist in the module's resource store
3. Allows specific cross-module ServiceAccount references (prometheus, grafana, log-shipper)
4. Ensures ServiceAccounts in the same namespace as the module are properly defined

**Why it matters:**

Binding to non-existent ServiceAccounts creates broken RBAC configurations that fail silently. This rule ensures all ServiceAccount references are valid, preventing runtime access control failures and security gaps.

**Examples:**

❌ **Incorrect** - Binding to non-existent ServiceAccount:

```yaml
# templates/rbac-for-us.yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: d8-my-module
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:my-module:my-app
subjects:
  - kind: ServiceAccount
    name: other-app  # ❌ Doesn't exist in module
    namespace: d8-my-module
roleRef:
  kind: ClusterRole
  name: d8:my-module:my-app
  apiGroup: rbac.authorization.k8s.io
```

**Error:**
```
Error: ClusterRoleBinding bind to the wrong ServiceAccount (doesn't exist in the store)
```

❌ **Incorrect** - Typo in ServiceAccount name:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: application
  namespace: d8-my-module
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: application-binding
  namespace: d8-my-module
subjects:
  - kind: ServiceAccount
    name: app  # ❌ Typo - should be "application"
    namespace: d8-my-module
roleRef:
  kind: Role
  name: application
  apiGroup: rbac.authorization.k8s.io
```

**Error:**
```
Error: RoleBinding bind to the wrong ServiceAccount (doesn't exist in the store)
```

✅ **Correct** - Binding to existing ServiceAccount:

```yaml
# templates/rbac-for-us.yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: d8-my-module
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:my-module:my-app
subjects:
  - kind: ServiceAccount
    name: my-app  # ✅ Exists in module
    namespace: d8-my-module
roleRef:
  kind: ClusterRole
  name: d8:my-module:my-app
  apiGroup: rbac.authorization.k8s.io
```

✅ **Correct** - Multiple ServiceAccounts:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: controller
  namespace: d8-my-module
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: webhook
  namespace: d8-my-module
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:my-module:combined
subjects:
  - kind: ServiceAccount
    name: controller
    namespace: d8-my-module
  - kind: ServiceAccount
    name: webhook
    namespace: d8-my-module
roleRef:
  kind: ClusterRole
  name: d8:my-module:combined
  apiGroup: rbac.authorization.k8s.io
```

✅ **Correct** - Allowed cross-module ServiceAccount (prometheus):

```yaml
# Binding to Prometheus for metrics scraping
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:my-module:prometheus
subjects:
  - kind: ServiceAccount
    name: prometheus  # ✅ Allowed cross-module reference
    namespace: d8-monitoring
roleRef:
  kind: ClusterRole
  name: d8:my-module:prometheus
  apiGroup: rbac.authorization.k8s.io
```

**Allowed Cross-Module ServiceAccounts:**
- `prometheus` in `d8-monitoring` - For metrics scraping
- `grafana` in `d8-monitoring` (when module is `loki`) - For log integration
- `log-shipper` in `d8-log-shipper` (when module is `loki`) - For log collection

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      binding-subject:
        - cdi-sa                                        # Exclude specific ServiceAccount name
        - kubevirt-internal-virtualization-controller
        - kubevirt-internal-virtualization-handler
```

---

### placement

**Purpose:** Enforces strict file placement and naming conventions for RBAC resources. This ensures consistent organization, prevents naming conflicts, and makes RBAC structure predictable across all Deckhouse modules.

**Description:**

Validates that RBAC resources (ServiceAccounts, Roles, ClusterRoles, RoleBindings, ClusterRoleBindings) are placed in correct files with proper naming conventions based on their purpose and scope.

**What it checks:**

1. **ServiceAccount Placement:**
   - Root level: `templates/rbac-for-us.yaml`
   - Nested: `templates/**/rbac-for-us.yaml`
   - Validates naming based on namespace and hierarchy

2. **ClusterRole/ClusterRoleBinding Placement:**
   - User authz: `templates/user-authz-cluster-roles.yaml`
   - Module-level: `templates/rbac-for-us.yaml` or `templates/**/rbac-for-us.yaml`
   - RBACv2: `templates/rbac/`

3. **Role/RoleBinding Placement:**
   - For module: `templates/rbac-for-us.yaml` or `templates/**/rbac-for-us.yaml`
   - From module: `templates/rbac-to-us.yaml` or `templates/**/rbac-to-us.yaml`

4. **Naming Conventions:**
   - Validates prefixes based on scope (local, global, system)
   - Ensures consistent delimiter usage (`:` for ClusterRoles, `-` for ServiceAccounts)

**Why it matters:**

Consistent RBAC organization makes modules predictable, reduces errors, prevents naming conflicts, and enables automated tooling to understand and manage RBAC resources. Proper placement also clarifies the scope and purpose of each RBAC resource.

**ServiceAccount Naming Logic:**

**For Root Level (`templates/rbac-for-us.yaml`):**
- **Simple name**: `<module-name>` → Deploy to module namespace (`d8-<module-name>`)
- **System name**: `d8-<module-name>` → Deploy to system namespaces (`d8-system`, `d8-monitoring`, etc.)

**For Nested Paths (`templates/**/rbac-for-us.yaml`):**
1. **Extract path components**: `templates/<path>/rbac-for-us.yaml` → `["<path>", "parts"]`
2. **Join with hyphens**: `path-parts` (base ServiceAccount name)
3. **Full name**: `<module-name>-<base-name>` (for system namespaces)

**Examples of Path to Name Conversion:**
- `templates/webhook/rbac-for-us.yaml` → `webhook` (local) or `module-webhook` (system)
- `templates/images/cdi/cdi-operator/rbac-for-us.yaml` → `images-cdi-cdi-operator` (local) or `module-images-cdi-cdi-operator` (system)
- `templates/controller/webhook/rbac-for-us.yaml` → `controller-webhook` (local) or `module-controller-webhook` (system)

**Role Naming Logic:**

**For ClusterRoles:**
- Root: `d8:<module-name>:<suffix>`
- Nested: `d8:<module-name>:<path>:<suffix>` (path joined with `:`)

**For Roles in rbac-for-us.yaml:**
- Local scope: `<name>` (in module namespace)
- Global scope: `d8:<module-name>:<suffix>` (in system namespaces)

**For Roles in rbac-to-us.yaml:**
- `access-to-<module-name>-<suffix>`

**ServiceAccount Placement Rules:**

❌ **Incorrect** - ServiceAccount in wrong file:

```yaml
# templates/deployment.yaml (wrong file)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: d8-my-module
```

**Error:**
```
Error: ServiceAccount should be in "templates/rbac-for-us.yaml" or "*/rbac-for-us.yaml"
```

❌ **Incorrect** - Wrong ServiceAccount name in system namespace:

```yaml
# templates/rbac-for-us.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-module  # ❌ Should be d8-my-module in system namespace
  namespace: kube-system
```

**Error:**
```
Error: Name of ServiceAccount in "templates/rbac-for-us.yaml" in namespace "kube-system" should be equal to d8- + Chart Name (d8-my-module)
```

✅ **Correct** - ServiceAccount in module namespace:

```yaml
# templates/rbac-for-us.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-module
  namespace: d8-my-module
```

✅ **Correct** - ServiceAccount in system namespace:

```yaml
# templates/rbac-for-us.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d8-my-module
  namespace: kube-system
```

✅ **Correct** - Nested ServiceAccount:

```yaml
# templates/controller/rbac-for-us.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: controller
  namespace: d8-my-module
```

Or with module prefix:

```yaml
# templates/controller/rbac-for-us.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-module-controller
  namespace: d8-system
```

**Common Error Scenarios:**

❌ **Incorrect** - ServiceAccount name doesn't match path structure:

```yaml
# File: templates/pre-delete-hook/rbac-for-us.yaml
# Path analysis: templates/pre-delete-hook/rbac-for-us.yaml
# Expected: parts = ["pre-delete-hook"] → serviceAccountName = "pre-delete-hook"
# Expected names: "pre-delete-hook" (local) or "module-pre-delete-hook" (system)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pre-delete-hook-sa  # ❌ Wrong name - doesn't match expected pattern
  namespace: d8-module
```

**Error:**
```
Error: Name of ServiceAccount should be equal to "pre-delete-hook" or "module-pre-delete-hook"
```

❌ **Incorrect** - Wrong namespace for system ServiceAccount name:

```yaml
# File: templates/controller/rbac-for-us.yaml
# Expected names: "controller" (local) or "module-controller" (system)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: module-controller  # ✅ Name matches system pattern
  namespace: d8-module     # ❌ Wrong namespace - should be d8-system or d8-monitoring
```

**Error:**
```
Error: ServiceAccount should be deployed to "d8-system" or "d8-monitoring"
```

✅ **Correct** - Matching path and namespace:

```yaml
# File: templates/pre-delete-hook/rbac-for-us.yaml
# Path: pre-delete-hook → serviceAccountName = "pre-delete-hook"
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pre-delete-hook        # ✅ Local name
  namespace: d8-module         # ✅ Module namespace
```

```yaml
# File: templates/pre-delete-hook/rbac-for-us.yaml
# Path: pre-delete-hook → expectedServiceAccountName = "module-pre-delete-hook"
apiVersion: v1
kind: ServiceAccount
metadata:
  name: module-pre-delete-hook # ✅ System name
  namespace: d8-system         # ✅ System namespace
```

**ClusterRole Placement Rules:**

❌ **Incorrect** - ClusterRole in wrong file:

```yaml
# templates/deployment.yaml (wrong file)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:controller
```

**Error:**
```
Error: ClusterRole should be in "templates/user-authz-cluster-roles.yaml" or "templates/rbac-for-us.yaml" or "*/rbac-for-us.yaml"
```

❌ **Incorrect** - ClusterRole with wrong name prefix:

```yaml
# templates/rbac-for-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-module:controller  # ❌ Should start with d8:my-module
```

**Error:**
```
Error: Name of ClusterRole in "templates/rbac-for-us.yaml" should start with "d8:my-module"
```

✅ **Correct** - ClusterRole with proper prefix:

```yaml
# templates/rbac-for-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:controller
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
```

✅ **Correct** - Nested ClusterRole:

```yaml
# File: templates/webhook/rbac-for-us.yaml
# Path: templates/webhook/rbac-for-us.yaml
# Expected prefix: d8:my-module:webhook
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:webhook:handler  # ✅ Follows d8:<module>:<path>:<suffix> pattern
rules:
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations"]
    verbs: ["get", "list", "watch"]
```

❌ **Incorrect** - Nested ClusterRole with wrong prefix:

```yaml
# File: templates/controller/webhook/rbac-for-us.yaml
# Path: templates/controller/webhook/rbac-for-us.yaml
# Expected prefix: d8:my-module:controller:webhook
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:webhook-handler  # ❌ Wrong prefix - missing "controller" part
rules:
  - apiGroups: ["admissionregistration.k8s.io"]
    resources: ["validatingwebhookconfigurations"]
    verbs: ["get", "list", "watch"]
```

**Error:**
```
Error: Name of ClusterRole should start with "d8:my-module:controller:webhook"
```

**Role Placement Rules:**

❌ **Incorrect** - Role with wrong prefix in rbac-to-us.yaml:

```yaml
# templates/rbac-to-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-module-access  # ❌ Should start with access-to-my-module
  namespace: d8-my-module
```

**Error:**
```
Error: Role in "templates/rbac-to-us.yaml" should start with "access-to-my-module"
```

✅ **Correct** - Role in rbac-for-us.yaml:

```yaml
# templates/rbac-for-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-module
  namespace: d8-my-module
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get"]
```

✅ **Correct** - Role in rbac-to-us.yaml:

```yaml
# templates/rbac-to-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: access-to-my-module-config
  namespace: d8-my-module
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["my-module-config"]
    verbs: ["get"]
```

**Naming Patterns Summary:**

| Resource Type | File | Namespace | Name Pattern |
|---------------|------|-----------|--------------|
| ServiceAccount | rbac-for-us.yaml | Module namespace | `<module-name>` |
| ServiceAccount | rbac-for-us.yaml | System namespace | `d8-<module-name>` |
| ServiceAccount | rbac-for-us.yaml | d8-system/d8-monitoring | `<module-name>` or `d8-<module-name>` |
| ClusterRole | rbac-for-us.yaml | N/A | `d8:<module-name>:<suffix>` |
| ClusterRole | nested/rbac-for-us.yaml | N/A | `d8:<module-name>:<path>:<suffix>` |
| Role | rbac-for-us.yaml | Module namespace | `<name>` |
| Role | rbac-for-us.yaml | d8-system/d8-monitoring | `d8:<module-name>:<suffix>` |
| Role | rbac-to-us.yaml | Any | `access-to-<module-name>-<suffix>` |

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      placement:
        - kind: ClusterRole
          name: d8:rbac-proxy
        - kind: ServiceAccount
          name: special-case
```

---

### wildcards

**Purpose:** Prevents use of wildcard (`*`) permissions in Roles and ClusterRoles. Wildcards grant overly broad access that violates the principle of least privilege and creates security risks.

**Description:**

Validates that Roles and ClusterRoles in `rbac-for-us.yaml` files don't use wildcard (`*`) in apiGroups, resources, or verbs. Each permission should be explicitly listed to ensure clear understanding of granted access.

**What it checks:**

1. Scans Roles and ClusterRoles in files ending with `rbac-for-us.yaml`
2. Checks each rule for wildcards in:
   - `apiGroups` - Should list specific API groups
   - `resources` - Should list specific resource types
   - `verbs` - Should list specific actions

**Why it matters:**

Wildcard permissions grant excessive access that:
1. **Security Risk**: Provides more access than necessary, violating least privilege
2. **Audit Complexity**: Makes it harder to understand actual permissions
3. **Privilege Escalation**: Can enable unintended privilege escalation paths
4. **Compliance**: Fails security audits and compliance requirements

**Examples:**

❌ **Incorrect** - Wildcard in apiGroups:

```yaml
# templates/rbac-for-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:controller
rules:
  - apiGroups: ["*"]  # ❌ Wildcard in apiGroups
    resources: ["pods"]
    verbs: ["get", "list"]
```

**Error:**
```
Error: apiGroups contains a wildcards. Replace them with an explicit list of resources
```

❌ **Incorrect** - Wildcard in resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-module
  namespace: d8-my-module
rules:
  - apiGroups: [""]
    resources: ["*"]  # ❌ Wildcard in resources
    verbs: ["get"]
```

**Error:**
```
Error: resources contains a wildcards. Replace them with an explicit list of resources
```

❌ **Incorrect** - Wildcard in verbs:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:admin
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["*"]  # ❌ Wildcard in verbs
```

**Error:**
```
Error: verbs contains a wildcards. Replace them with an explicit list of resources
```

❌ **Incorrect** - Multiple wildcards:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:super-admin
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]  # ❌ All wildcards
```

**Error:**
```
Error: apiGroups, resources, verbs contains a wildcards. Replace them with an explicit list of resources
```

✅ **Correct** - Explicit permissions:

```yaml
# templates/rbac-for-us.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:controller
rules:
  - apiGroups: [""]
    resources: ["pods", "configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets"]
    verbs: ["get", "list", "watch", "update", "patch"]
```

✅ **Correct** - Multiple rules for clarity:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:operator
rules:
  # Core resources - read only
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  
  # Core resources - write access
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["create", "update", "patch", "delete"]
  
  # Apps resources
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # Custom resources
  - apiGroups: ["mymodule.deckhouse.io"]
    resources: ["myresources"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  # Status subresource
  - apiGroups: ["mymodule.deckhouse.io"]
    resources: ["myresources/status"]
    verbs: ["get", "update", "patch"]
```

✅ **Correct** - Admin role with explicit permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:my-module:admin
rules:
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - pods/exec
      - services
      - endpoints
      - configmaps
      - secrets
      - serviceaccounts
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
  - apiGroups: ["apps"]
    resources:
      - deployments
      - statefulsets
      - daemonsets
      - replicasets
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**When wildcards might seem necessary:**

If you find yourself wanting to use wildcards, consider:

1. **List all resources explicitly** - Better for security and auditability
2. **Create multiple specific roles** - Instead of one broad role
3. **Use aggregated ClusterRoles** - Kubernetes can aggregate multiple ClusterRoles
4. **Review actual needs** - Often you don't need as much access as you think

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      wildcards:
        - kind: ClusterRole
          name: d8:deckhouse:webhook-handler  # Exclude specific ClusterRole
        - kind: Role
          name: special-admin-role
```

## Configuration

The RBAC linter can be configured at the module level with rule-specific exclusions.

### Module-Level Settings

Configure the overall impact level for the rbac linter:

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### Rule-Level Exclusions

#### Binding Subject Exclusions

Exclude specific ServiceAccount names from validation:

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      binding-subject:
        - cdi-sa
        - kubevirt-internal-virtualization-controller
        - kubevirt-internal-virtualization-handler
        - external-service-account
```

#### Placement Exclusions

Exclude specific RBAC resources from placement validation:

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      placement:
        - kind: ClusterRole
          name: d8:rbac-proxy
        - kind: ServiceAccount
          name: special-webhook
        - kind: Role
          name: legacy-role
```

#### Wildcards Exclusions

Exclude specific Roles/ClusterRoles from wildcard validation:

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    exclude-rules:
      wildcards:
        - kind: ClusterRole
          name: d8:deckhouse:webhook-handler
        - kind: ClusterRole
          name: d8:my-module:legacy-admin
        - kind: Role
          name: debug-role
```

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  rbac:
    # Global impact level
    impact: error
    
    # Rule-specific exclusions
    exclude-rules:
      # Exclude ServiceAccount names from binding validation
      binding-subject:
        - cdi-sa
        - kubevirt-internal-virtualization-controller
        - kubevirt-internal-virtualization-handler
      
      # Exclude specific resources from placement validation
      placement:
        - kind: ClusterRole
          name: d8:rbac-proxy
        - kind: ServiceAccount
          name: special-case-sa
      
      # Exclude specific resources from wildcard validation
      wildcards:
        - kind: ClusterRole
          name: d8:deckhouse:webhook-handler
        - kind: ClusterRole
          name: d8:my-module:admin
```

### Configuration in Module Directory

You can also place a `.dmt.yaml` configuration file directly in your module directory:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  rbac:
    impact: warning  # More lenient for this specific module
    exclude-rules:
      binding-subject:
        - legacy-sa
      wildcards:
        - kind: ClusterRole
          name: d8:my-module:legacy-admin
```

## Common Issues

### Issue: ServiceAccount binding validation failure

**Symptom:**
```
Error: ClusterRoleBinding bind to the wrong ServiceAccount (doesn't exist in the store)
```

**Cause:** RoleBinding or ClusterRoleBinding references a ServiceAccount that doesn't exist in the module.

**Solutions:**

1. **Create the missing ServiceAccount:**

   ```yaml
   # templates/rbac-for-us.yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: my-app
     namespace: d8-my-module
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: d8:my-module:my-app
   subjects:
     - kind: ServiceAccount
       name: my-app
       namespace: d8-my-module
   roleRef:
     kind: ClusterRole
     name: d8:my-module:my-app
     apiGroup: rbac.authorization.k8s.io
   ```

2. **Fix typo in ServiceAccount name:**

   Ensure the name in subjects matches the ServiceAccount definition exactly.

3. **Exclude the ServiceAccount from validation:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     rbac:
       exclude-rules:
         binding-subject:
           - external-service-account
   ```

### Issue: ClusterRole placement error

**Symptom:**
```
Error: ClusterRole should be in "templates/user-authz-cluster-roles.yaml" or "templates/rbac-for-us.yaml" or "*/rbac-for-us.yaml"
```

**Cause:** ClusterRole is defined in a file other than the allowed locations.

**Solutions:**

1. **Move ClusterRole to proper file:**

   ```bash
   # Move to rbac-for-us.yaml
   # Ensure ClusterRole is in templates/rbac-for-us.yaml
   ```

2. **Use correct file for user-authz ClusterRoles:**

   ```yaml
   # templates/user-authz-cluster-roles.yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:user-authz:my-module:admin
     annotations:
       user-authz.deckhouse.io/access-level: Admin
   ```

3. **Use rbac-for-us.yaml for module ClusterRoles:**

   ```yaml
   # templates/rbac-for-us.yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:my-module:controller
   ```

### Issue: ClusterRole name doesn't follow convention

**Symptom:**
```
Error: Name of ClusterRole in "templates/rbac-for-us.yaml" should start with "d8:my-module"
```

**Cause:** ClusterRole name doesn't follow the required naming pattern.

**Solutions:**

1. **Fix the ClusterRole name:**

   ```yaml
   # Before
   metadata:
     name: my-module-controller
   
   # After
   metadata:
     name: d8:my-module:controller
   ```

2. **For nested rbac-for-us.yaml:**

   ```yaml
   # templates/webhook/rbac-for-us.yaml
   # Before
   metadata:
     name: webhook-handler
   
   # After
   metadata:
     name: d8:my-module:webhook:handler
   ```

### Issue: Wildcard permissions detected

**Symptom:**
```
Error: apiGroups, resources, verbs contains a wildcards. Replace them with an explicit list of resources
```

**Cause:** Role or ClusterRole uses wildcard (`*`) permissions.

**Solutions:**

1. **Replace wildcards with explicit lists:**

   ```yaml
   # Before
   rules:
     - apiGroups: ["*"]
       resources: ["*"]
       verbs: ["*"]
   
   # After
   rules:
     - apiGroups: [""]
       resources: ["pods", "configmaps", "secrets"]
       verbs: ["get", "list", "watch", "create", "update", "delete"]
     - apiGroups: ["apps"]
       resources: ["deployments", "statefulsets"]
       verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
   ```

2. **Create separate rules for different resource groups:**

   ```yaml
   rules:
     # Read access to core resources
     - apiGroups: [""]
       resources: ["pods", "services", "endpoints"]
       verbs: ["get", "list", "watch"]
     
     # Write access to config resources
     - apiGroups: [""]
       resources: ["configmaps", "secrets"]
       verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
     
     # Full access to custom resources
     - apiGroups: ["mymodule.deckhouse.io"]
       resources: ["myresources"]
       verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
   ```

3. **Exclude if wildcards are absolutely necessary (not recommended):**

   ```yaml
   # .dmt.yaml
   linters-settings:
     rbac:
       exclude-rules:
         wildcards:
           - kind: ClusterRole
             name: d8:my-module:legacy-admin
   ```

### Issue: ServiceAccount name and namespace mismatch in nested paths

**Symptom:**
```
Error: ServiceAccount should be deployed to "d8-system" or "d8-monitoring"
```
or
```
Error: Name of ServiceAccount should be equal to "path-parts" or "module-path-parts"
```

**Cause:** ServiceAccount name doesn't match the expected pattern based on file path structure, or namespace doesn't correspond to the name type.

**Path Analysis Logic:**
1. **Extract path**: `templates/<folder>/rbac-for-us.yaml`
2. **Split by `/`**: Get path components
3. **Join with `-`**: Create base ServiceAccount name
4. **Check name pattern**:
   - Local name (`path-parts`) → Module namespace (`d8-<module>`)
   - Global name (`<module>-path-parts`) → System namespaces (`d8-system`, `d8-monitoring`)

**Examples:**

**File:** `templates/pre-delete-hook/rbac-for-us.yaml`
- **Path parts**: `["pre-delete-hook"]`
- **Base name**: `"pre-delete-hook"`
- **Expected names**:
  - `"pre-delete-hook"` → namespace: `d8-<module>`
  - `"<module>-pre-delete-hook"` → namespace: `d8-system` or `d8-monitoring`

**File:** `templates/images/cdi/cdi-operator/rbac-for-us.yaml`
- **Path parts**: `["images", "cdi", "cdi-operator"]`
- **Base name**: `"images-cdi-cdi-operator"`
- **Expected names**:
  - `"images-cdi-cdi-operator"` → namespace: `d8-<module>`
  - `"<module>-images-cdi-cdi-operator"` → namespace: `d8-system` or `d8-monitoring`

**Solutions:**

1. **Use local name in module namespace:**
   ```yaml
   # templates/pre-delete-hook/rbac-for-us.yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: pre-delete-hook      # ✅ Local pattern
     namespace: d8-my-module    # ✅ Module namespace
   ```

2. **Use global name in system namespace:**
   ```yaml
   # templates/pre-delete-hook/rbac-for-us.yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: my-module-pre-delete-hook  # ✅ Global pattern
     namespace: d8-system             # ✅ System namespace
   ```

### Issue: ServiceAccount wrong namespace for placement

**Symptom:**
```
Error: ServiceAccount in "templates/rbac-for-us.yaml" should be deployed in namespace "d8-my-module"
```

**Cause:** ServiceAccount is deployed to wrong namespace based on its name.

**Solutions:**

1. **Fix namespace to match module:**

   ```yaml
   # templates/rbac-for-us.yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: my-module
     namespace: d8-my-module  # Match module namespace
   ```

2. **Use d8- prefix for system namespaces:**

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: d8-my-module  # Add d8- prefix
     namespace: kube-system
   ```

### Issue: User-authz ClusterRole missing annotation

**Symptom:**
```
Error: User-authz access ClusterRoles should have annotation "user-authz.deckhouse.io/access-level"
```

**Cause:** ClusterRole in user-authz file doesn't have required annotation.

**Solutions:**

1. **Add the access-level annotation:**

   ```yaml
   # templates/user-authz-cluster-roles.yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: d8:user-authz:my-module:admin
     annotations:
       user-authz.deckhouse.io/access-level: Admin  # Add this
   rules:
     - apiGroups: [""]
       resources: ["pods"]
       verbs: ["*"]
   ```

2. **Ensure name matches access level:**

   ```yaml
   # Access level "Editor" becomes "editor" in name
   metadata:
     name: d8:user-authz:my-module:editor
     annotations:
       user-authz.deckhouse.io/access-level: Editor
   ```
