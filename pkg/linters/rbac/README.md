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
| [contract](#contract) | Holds the module's RBACv2 roles and capabilities to the platform's label and naming contract | ✅ | enabled |
| [coverage](#coverage) | Requires a decision in `rbac.yaml` on the user access to every CRD the module ships | ✅ | enabled when `rbac.yaml` exists |
| [sync](#sync) | Compares the rendered RBAC objects with `rbac.yaml` in both directions; `--fix` regenerates the templates, and writes the first `rbac.yaml` from the render of a module that has none | ✅ | always on |

"Configurable" means that this rule can be configured using the `.dmtlint.yaml` file, including customizing the rule's parameters and/or disabling the rule.

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
# .dmtlint.yaml
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
# Expected names: "pre-delete-hook" (local) or "my-module-pre-delete-hook" (system)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pre-delete-hook-sa  # ❌ Wrong name - doesn't match expected pattern
  namespace: d8-my-module
```

**Error:**
```
Error: Name of ServiceAccount should be equal to "pre-delete-hook" or "my-module-pre-delete-hook"
```

❌ **Incorrect** - Wrong namespace for system ServiceAccount name:

```yaml
# File: templates/controller/rbac-for-us.yaml
# Expected names: "controller" (local) or "my-module-controller" (system)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-module-controller  # ✅ Name matches system pattern
  namespace: d8-my-module     # ❌ Wrong namespace - should be d8-system or d8-monitoring
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
  namespace: d8-my-module      # ✅ Module namespace
```

```yaml
# File: templates/pre-delete-hook/rbac-for-us.yaml
# Path: pre-delete-hook → expectedServiceAccountName = "my-module-pre-delete-hook"
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-module-pre-delete-hook # ✅ System name
  namespace: d8-system            # ✅ System namespace
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
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

You can also place a `.dmtlint.yaml` configuration file directly in your module directory:
```yaml
# modules/my-module/.dmtlint.yaml
linters-settings:
  rbac:
    impact: warn  # More lenient for this specific module; ignored silences every rbac rule the root does not set
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
   # .dmtlint.yaml
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
   # .dmtlint.yaml
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

---

## The module RBAC declaration: `rbac.yaml`

The three rules below work with one file, `modules/<module>/rbac.yaml`: the single machine-readable
source of the RBAC a module ships. It describes, per resource, the access every role model grants
(the RBACv2 namespace and system lineages, and the legacy user-authz access levels), the rights of the
module's ServiceAccounts, and the access other components get to the module. The templates under
`templates/rbacv2/`, `templates/user-authz-cluster-roles.yaml`, `templates/**/rbac-for-us.yaml` and
`templates/rbac-to-us.yaml` are **generated** from it by `dmt lint --linter rbac --fix`.

```yaml
# modules/<module>/rbac.yaml
apiVersion: rbac.deckhouse.io/v1alpha1

# Lineages the system capabilities aggregate into. Defaults to `subsystems` of module.yaml; required
# when the module aggregates into more subsystems than module.yaml declares.
subsystems: [networking, kubernetes]

resources:
  # A resource the module ships a CRD for: group and resource are enough, the scope comes from the CRD.
  - group: cert-manager.io
    resource: certificates
    namespace:                      # RBACv2 namespace lineage; only for Namespaced resources
      viewer: [get, list, watch]
      manager: [create, update, patch, delete, deletecollection]
    legacy:                         # user-authz v1; never derived from the RBACv2 levels
      User: [get, list, watch]
      Editor: [create, update, patch, delete, deletecollection]

  # A cluster-scoped resource goes to the system lineage; a Namespaced one may too, with a reason.
  - group: cert-manager.io
    resource: clusterissuers
    system:
      viewer: [get, list, watch]
      manager: [create, update, patch, delete, deletecollection]
    legacy:
      ClusterEditor: [create, update, patch, delete, deletecollection]

  # A resource the module ships no CRD for: declare the scope; its existence is not checked.
  - group: trivy.deckhouse.io
    resource: vulnerabilityreports
    scope: Namespaced
    namespace:
      viewer: [get, list, watch]

  # A whole group whose resources are created at runtime: "*" with a reason.
  - group: constraints.gatekeeper.sh
    resource: "*"
    scope: Cluster
    reason: one CRD per ConstraintTemplate is created at runtime; the names are not known statically
    system:
      viewer: [get, list, watch]

  # A deliberate denial: the reason is for the reader. It excludes namespace, system and legacy.
  - group: deckhouse.io
    resource: registryscantargets
    noAccess: internal resource, managed by the controller

  # A conditional grant: `when` is a Helm expression, rendered as {{- if <when> }} around the rule.
  - group: deckhouse.io
    resource: bars
    when: .Values.foo.barEnabled
    namespace:
      viewer: [get, list, watch]

# Localized texts of capabilities outside the view/edit convention (their texts come from the platform).
capabilities:
  namespace.admin:
    title: {en: "Module cert-manager: admin", ru: "Модуль cert-manager: администрирование"}
    description: {en: "Manage cert-manager Issuers in a namespace.", ru: "Управление Issuer модуля cert-manager в пространстве имён."}

# ServiceAccount rights -> templates/[<path>/]rbac-for-us.yaml. Only declared accounts are managed.
# automountServiceAccountToken defaults to false; extraClusterRoles are further ClusterRoles in the
# account's file, d8:<module>:<account>:<name> (or exactly the name when it starts with d8:), bound
# to the account unless bind: false -- roles split by concern, or roles shipped for others to bind.
serviceAccounts:
  - name: cainjector
    path: cainjector                # templates/cainjector/rbac-for-us.yaml; omitted -> templates/rbac-for-us.yaml
                                    # a nested path a/b names the account a-b or <module>-a-b (placement rule)
    when: .Values.certManager.internal.enableCAInjector
    labels: {app: cainjector}
    annotations: {helm.sh/resource-policy: keep}    # on the ServiceAccount
    rbacAnnotations: {werf.io/deploy-on: pre-install} # on every role and binding of the account
    clusterRules:                   # ClusterRole d8:<module>:<name> + ClusterRoleBinding
      - apiGroups: [cert-manager.io]
        resources: [certificates]
        verbs: [get, list, watch]
      - nonResourceURLs: [/metrics]
        verbs: [get]
    namespaceRules:                 # Role <name> in the module namespace + RoleBinding
      - apiGroups: [coordination.k8s.io]
        resources: [leases]
        verbs: [get, list, watch, create, update, patch]
    bindClusterRoles: [d8:rbac-proxy]         # ClusterRoleBinding d8:<module>:<name>:rbac-proxy
    bindRoles:                                # RoleBinding to an existing Role in a foreign namespace
      - namespace: kube-system
        name: extension-apiserver-authentication-reader

# Metrics access -> templates/rbac-to-us.yaml (Role/RoleBinding access-to-<module>); `when` gates the
# RoleBinding to the scraper only, the Role is unconditional, as the modules write it today
prometheusAccess:
  deployments: [cert-manager]
  when: .Values.global.enabledModules | has "prometheus"

# Arbitrary subjects: clusterRules -> templates/[<path>/]rbac-for-us.yaml, namespaceRules -> templates/[<path>/]rbac-to-us.yaml
access:
  - name: admin-kubeconfig
    when: .Values.certManager.adminKubeconfig   # optional; wraps the role and the binding, as for an account
    subjects:
      - kind: Group
        name: kubeadm:cluster-admins
    clusterRules:
      - apiGroups: [cert-manager.io]
        resources: [clusterissuers]
        verbs: [get, list, watch, create, update, patch, delete]
```

Format rules the loader enforces:

- `apiVersion` is required and must be `rbac.deckhouse.io/v1alpha1`; unknown keys anywhere are an error.
- Verbs are listed explicitly (`get`, `list`, `watch`, `create`, `update`, `patch`, `delete`, `deletecollection`); there are no aliases, and `*` is refused at every level (the `contract` rule reports `*` verbs and `apiGroups: ["*"]` in a rendered capability, too). `group: "*"` is refused; a group is a lowercase DNS name and a resource a lowercase plural with an optional `/<subresource>`, where `*` and `*/<subresource>` are the only wildcards.
- No value holds `{{` or `}}`: the generator writes the values into Helm templates as they are, and a `when` is the condition only.
- `legacy.SuperAdmin` validates but is a warning: user-authz aggregates custom legacy roles for `User` through `ClusterAdmin` only.
- `prometheusAccess.when` gates the RoleBinding of the Prometheus scraper (typically `.Values.global.enabledModules | has "prometheus"`); the Role stays unconditional.
- Levels: `namespace` -- `viewer`, `user`, `manager`, `admin`, `superadmin`; `system` -- `viewer`, `manager`, `superadmin`; `legacy` -- `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`.
- `namespace` levels are allowed only for `Namespaced` resources; a `Namespaced` resource at a `system` level needs a `reason`.
- `scope` is required for a resource the module ships no CRD for, and must agree with the CRD when the module ships one. `resource: "*"` is allowed only for a group without CRDs in the module and needs a `reason`.
- `noAccess` is a non-empty reason and excludes the levels. `noAccess: "TODO"` is the stub the coverage autofix writes; it is not a decision. Any `noAccess`, `reason` or `scope` value that starts with `TODO` is an open decision: `coverage` reports it, and a `--fix` run that meets one exits non-zero.
- A capability outside the view/edit convention (`admin`, `user`, `superadmin`) needs `capabilities.<lineage>.<level>` texts in both languages.

What the generator produces from it (level `viewer` -> capability `view`, `manager` -> `edit`, the rest as they are):

| Section | File | Objects |
|---|---|---|
| `resources[].namespace.<level>` | `templates/rbacv2/use/<action>.yaml` | ClusterRole `d8:namespace-capability:<module>:<action>` |
| `resources[].system.<level>` | `templates/rbacv2/manage/<action>.yaml` | ClusterRole `d8:system-capability:<module>:<action>`; `view` and `edit` are always produced, with the rule on the module's own ModuleConfig |
| `resources[].legacy.<Level>` | `templates/user-authz-cluster-roles.yaml` | ClusterRole `d8:user-authz:<module>:<kebab-level>` with the `user-authz.deckhouse.io/access-level` annotation |
| `serviceAccounts[]` | `templates/[<path>/]rbac-for-us.yaml` | ServiceAccount, ClusterRole/ClusterRoleBinding `d8:<module>:<name>`, Role/RoleBinding `<name>`, the extra bindings |
| `access[]` with `clusterRules` | `templates/rbac-for-us.yaml` | ClusterRole/ClusterRoleBinding `d8:<module>:<name>` |
| `prometheusAccess`, `access[]` with `namespaceRules` | `templates/[<path>/]rbac-to-us.yaml` | Role/RoleBinding `access-to-<module>[-<name>]`; with `path` `access-to-<path, / as ->-<name>`, as the placement rule wants |

Every object gets the `rbac.deckhouse.io/namespace` label of the module namespace unless that namespace is
`default`: user-authz projects the module's use roles by it, and `kube-system` is a module namespace as
any `d8-*` one.

Every generated file starts with a header line naming the generator and the contract version. A file
without that header is maintained by hand and is never overwritten.

The developer's loop is: edit `rbac.yaml` -> `dmt lint --linter rbac --fix` -> `dmt lint`. `--fix` does not
re-lint: the second `dmt lint` shows the state after the fixes.

---

### contract

**Purpose:** Holds the RBACv2 roles and capabilities a module renders under `templates/rbacv2/` to the
platform's label and naming contract, so that a module outside the platform repository is checked
the same way the platform's own test (`testing/rbacv2`) checks in-tree modules. Role aggregation
relies on labels the API server cannot validate; a divergent module silently breaks it.

The standalone helper role `d8:dict`, which the `handle_dict_bindings` hook binds, is not an RBACv2
role or capability and is exempt from the naming and label checks.

**Description:**

Works on the rendered ClusterRoles from `templates/rbacv2/` (the compatibility aliases under
`templates/rbacv2-compat/` are outside the contract by design). Needs no `rbac.yaml`.

**What it checks:**

1. The name starts with `d8:`; the four `en|ru.meta.deckhouse.io/title|description` annotations are present; the `module` label is the module's name (the platform test cannot know the module; dmt does).
2. `rbac.deckhouse.io/kind` is `role` or `capability`; `rbac.deckhouse.io/scope` is `system`, `subsystem`, `namespace` or `project`.
3. A role: its name matches the pattern of its scope, it defines no `rules`, its `aggregationRule` selects only by `aggregate-to-<lineage>-as` labels with a known lineage and a level of that lineage; system/subsystem roles carry `rbac.deckhouse.io/use-role` with a valid level.
4. A capability: its name starts with the prefix of its scope, it defines `rules` and no `aggregationRule`, carries at least one `aggregate-to-<lineage>-as` label and a valid `rbac.deckhouse.io/capability` marker (a label value, at most 63 characters).
5. Aggregation labels target a known lineage (`system`, `namespace`, `project` or one of the seven subsystems) with a level that lineage has.
6. `rbac.deckhouse.io/delegatable` appears only on namespace/project roles.
7. An object of the RBACv2 scheme before DKP 1.78 (`rbac.deckhouse.io/kind: use` or `manage`, names `d8:use:capability:module:<m>:*` / `d8:manage:permission:module:<m>:*`) gets one finding -- migrate with `rbacv2-migrate-module.sh` from `modules/140-user-authz/docs/internal/` of the deckhouse repository, or describe the module in `rbac.yaml` and run `--fix` -- instead of failing every check above. A legacy object rendered from a template that carries the script's version gate (`include "<module>.rbacv2_new_scheme"`) is not reported: the module serves both models on purpose.
8. **Warning:** a cluster-scoped resource inside a namespace capability. Such a capability is bound through a RoleBinding, where the rule grants nothing. The scope comes from the module's CRDs or from its `rbac.yaml`; a resource the run knows nothing about is not judged.

What deliberately stays in the platform test: the levels of sensitive capabilities and the closure of
aggregation across two modules, and the global uniqueness of the capability marker -- a rule sees one module.

**Example finding:**

```
Error: capability "d8:namespace-capability:my-module:view" must carry the rbac.deckhouse.io/capability label
Warning: capability "d8:namespace-capability:my-module:view" grants my.io/globals, a cluster-scoped resource, in a namespace capability: bound through a RoleBinding the rule grants nothing; move it to a system capability
```

**Configuration:**
```yaml
# root .dmtlint.yaml
global:
  linters-settings:
    rbac:
      rules:
        contract: {impact: error}  # the level of this rule alone; unset it starts at warn, and the four original rules keep the linter level

# module .dmtlint.yaml
linters-settings:
  rbac:
    exclude-rules:
      contract:
        - kind: ClusterRole
          name: d8:namespace-capability:my-module:legacy
```

---

### coverage

**Purpose:** Every CRD a module ships gets a decision on the user access to it, written down in
`rbac.yaml`: the levels of either role model, or `noAccess` with the reason. Today 66 of 181 CRDs in
the platform are named in no user rule of their module, and nowhere is it recorded whether that is
deliberate.

**Description:**

Runs only when the module has an `rbac.yaml`. Reads the CRDs under `crds/` at any depth (selected by
`kind: CustomResourceDefinition`, so `crds/vendor/` counts and `images/**/testdata/crds/` does not).

**What it checks:**

1. Every CRD (`spec.group` / `spec.names.plural`) has an entry in `resources` that grants levels or denies access with a reason -- **error**, with an autofix.
2. An entry left as `noAccess: "TODO"` -- **error**, no autofix: only a person can decide.
3. An entry naming a group the module ships CRDs for, but a resource none of them spells -- **warning** (a likely misspelling). Whole-group (`"*"`) and subresource (`/`) entries are exempt.
4. A `noAccess` entry of a group the module ships no CRD for, without a `scope` -- **warning**: a removed CRD is indistinguishable from an external resource nobody grants. Add `scope` to say the resource is external, or drop the entry if its CRD is gone.

**Autofix:** appends an undecided stub for each CRD without an entry --

```yaml
  - group: deckhouse.io
    resource: foopolicies
    noAccess: "TODO"
```

-- and then still reports the finding: the stub is not a decision, and a `--fix` run that wrote stubs
does not end green. Existing entries and comments are left as they are; a second `--fix` changes nothing.
The rule does not create `rbac.yaml`: a module without the file is a `sync` finding, and `--fix` of
that rule writes the first declaration from the render (see [sync](#sync)).

**Example finding:**

```
Error: CRD deckhouse.io/foopolicies (crds/foopolicies.yaml) has no entry in rbac.yaml: decide the user access to it -- namespace, system or legacy levels, or noAccess with the reason; `dmt lint --linter rbac --fix` adds an undecided stub
```

**Configuration:**
```yaml
# root .dmtlint.yaml
global:
  linters-settings:
    rbac:
      rules:
        coverage: {impact: warn}

# module .dmtlint.yaml
linters-settings:
  rbac:
    exclude-rules:
      coverage:
        - deckhouse.io/internalthings   # "group/resource" of a CRD the declaration deliberately leaves out
```

---

### sync

**Purpose:** The rendered RBAC objects and `rbac.yaml` say the same thing, in both directions, so that
a reviewer reads one file to know what the module grants, and a change to the platform's contract is
a regeneration rather than a hand edit of every template.

**Description:**

Without `rbac.yaml` the rule reports the declaration missing, and `--fix` writes it from the RBAC
objects the module renders today: the declaration a person would have transcribed from the templates,
with a `TODO` wherever a decision is still theirs (a resource without a CRD whose scope the linter
cannot know, a CRD nobody grants, a namespaced resource granted cluster-wide) and a note on top for
every object the generator will name differently or cannot describe. An entry whose CRD is in `crds/`
carries no `scope`: the CRD states it; an external resource whose scope is not known gets
`scope: "TODO: Namespaced or Cluster (Cluster drops the namespace levels)"`. A grant limited to `resourceNames` is never widened to every
object: it is left out and named in a note. A role granting `*` verbs or API groups, which the format
refuses, is listed as hand-written with the reason.

The render only holds what rendered for the linter's values, so the importer also reads the template
text around each object. An object wrapped in `{{ if X }}` gets `when: X` (an `{{ else }}` branch
`not (X)`, nested blocks an `and`); a condition that uses a template variable becomes a `TODO`, and so
does an account or access entry whose role and binding render under different conditions. An object
inside `{{ range }}`, `{{ with }}` or `{{ define }}`, one rendered by an include of a named template
(`helm_lib_*`), and a role without rules stay hand-written. An object the text holds under a
condition false for these values is in no render: the header names it, and the fix fails, so the
regeneration does not drop it in silence. A block inside an object -- a rule under its own `{{ if }}`
-- is noted. The Prometheus scrape binding keeps its gate as `prometheusAccess.when`.

The fix that writes the file keeps the finding while a `TODO` is left in it or while the linter would
refuse the written file (both are named in the fix error), and a `--fix` run with any fix left open
exits non-zero whatever the level of its finding. Nothing is written into an edition overlay. Review
it, resolve the TODOs, then run `--fix` again to regenerate the templates from it. From then on
`rbac.yaml` is the source.

With `rbac.yaml` the rule first validates the declaration; a declaration with errors is reported and
nothing else is compared or generated. Then builds the objects the
declaration produces and compares them with the render.

`sync` owns exactly three classes of rendered objects:

1. legacy roles -- ClusterRoles with the `user-authz.deckhouse.io/access-level` annotation;
2. the module's own RBACv2 capabilities -- ClusterRoles named `d8:namespace-capability:<module>:<action>` or `d8:system-capability:<module>:<action>` with `rbac.deckhouse.io/kind: capability` and `module: <module>`. Capabilities of the project lineage and platform-wide ones named after a lineage rather than the module (user-authz, multitenancy-manager) are not the declaration's: the format has no place for them, so they stay hand-written;
3. declared objects -- those whose names the generator builds from `serviceAccounts`, `access` and `prometheusAccess`.

Everything else in the render is unmanaged: not generated, not reported (`include "helm_lib_csi_controller_rbac"`,
controller ClusterRoles with arbitrary names, objects with Helm-computed names).

**What it checks:**

1. Every declared object is in the render (unless it is under `when`), and every rule of it: rules are compared as `(apiGroup, resource, resourceName, verb)` tuples, in both directions. A rule under `when` that did not render is not a divergence; a rule without `when` hidden behind a hand-written `{{ if }}` is.
2. The annotations of an object written from `serviceAccounts` or `access` match the declaration's (`annotations`, `rbacAnnotations`): a `helm.sh/resource-policy: keep` the declaration does not carry would be lost by the next regeneration.
3. A capability's aggregation edges (`aggregate-to-<lineage>-as`) match in both directions: rules may agree while a lineage is lost. Its `rbac.deckhouse.io/capability` marker, `module` and `rbac.deckhouse.io/namespace` labels are what the generator writes.
4. A binding's `roleRef` and subjects match.
5. Every rendered legacy role and module capability is produced by the declaration.
6. A file the declaration produces that does not exist while an object it holds is absent from the render is a divergence, whether or not the object is under `when`: the render cannot tell a false condition from a template nobody wrote, the text can. A file that carries the generator header is the generator's, and its text must be what the declaration renders now: a rule under `when` whose condition is false today is absent from the render without being a divergence, yet it still has to reach the template, so for generator-owned files the text is compared too. A file of another contract version is the same case. Remove the header to maintain a file by hand; then only its render is judged.

Findings are one per template file and carry the fix command; the text does not depend on the render variant.
A declaration that does not parse or validate, one the generator cannot turn into objects (an account
named against the placement rule), a broken `module.yaml` and a declaration in an edition overlay stop
the rule; their fix fails, so `--fix` exits non-zero instead of reporting a run that generated nothing.

**Autofix:** regenerates the file from `rbac.yaml`. The declaration is the source of truth: a right it
no longer names leaves the template; the finding that led there listed it, and the autofix logs what it
removed, so a `--fix` run without a preceding `dmt lint` does not remove rights in silence. Three things are never
written over --

- a file that also holds objects of someone else -- a controller ClusterRole beside a declared ServiceAccount, a hand-written binding, a ConfigMap or a Secret -- is never rewritten, because the generator writes the whole file and they would vanish (and so they would if the file were deleted); the refusal names them: declare them (`extraClusterRoles`, `access` with `path`) or move them first. A generated file lists under its header, one `# dmt:owns <object>` line each, what the generator wrote into it (contract 2); an owned object the declaration no longer produces -- a legacy level dropped, a ServiceAccount removed -- is a removal, named in the finding and in the log, and everything else in the file is someone else's. The fix does not move objects between files: an object the declaration now puts in another file is refused in the file it renders from ("move it by hand, or delete this file"), and the target is not written while the object still renders elsewhere, so nothing is lost or rendered twice. Besides the render, the fix reads the objects of a generated file from its text, so an object under a condition that is false for these values -- a hand-added ConfigMap, an account moved elsewhere -- is judged too. In a file without the list (contract 1, or hand-written), a legacy role or a module capability the declaration does not produce is a removal too; any other object is refused, whatever its name. To drop an account or an access entry from a contract 1 file, run `--fix` once with the declaration unchanged -- that writes contract 2 -- and drop it then. An object the generator produces under another name -- a binding with the same roleRef and subjects, a role with the same rules, while the new name is not rendered yet -- is replaced, not foreign;
- a file without the generator header is maintained by hand: the generated text is written beside it as `_<file>.generated` (the underscore keeps Helm from rendering the copy) and the finding stays (delete the file and run `--fix` again to hand it back to the generator);
- a template that serves both role models behind the version gate (`rbacv2_new_scheme`) is never regenerated: the legacy branch would vanish;

A missing file is created. A generated file the declaration produces nothing for any more -- every namespace level dropped, the `legacy` section gone -- is deleted, as long as it holds nothing but what the generator owns; the deletion is logged. Such files are found on disk too (the `_<file>.generated` asides aside), so a file whose objects are all under a condition that is false for these values is not missed; one found only there is deleted only when its text holds nothing but what its header lists and the declaration places nowhere else. A template the tolerant render skipped in any render variant is neither compared nor regenerated: its objects were never seen. The removals the log names are the union over every render variant. A second `--fix` without changes to `rbac.yaml` changes nothing. Under
`--matrix` every render variant reports the file, but the fix runs once: the variants record what
their renders grant while they exist, the first closure checks the union and writes, the others
report its outcome -- so a right rendered only under some values is never dropped.

**Example finding:**

```
Error: templates/rbacv2/use/edit.yaml does not match rbac.yaml: ClusterRole/d8:namespace-capability:my-module:edit: get ""/secrets is in the render but not declared. Run `dmt lint --linter rbac --fix` to regenerate the file from the declaration
```

**Configuration:**
```yaml
# root .dmtlint.yaml
global:
  linters-settings:
    rbac:
      rules:
        sync: {impact: warn}

# module .dmtlint.yaml
linters-settings:
  rbac:
    exclude-rules:
      sync:
        - kind: ClusterRole
          name: d8:user-authz:my-module:super-admin
```

**Levels:** `contract`, `coverage` and `sync` start at `warn` wherever nothing sets them: they are new
to every tree. A tree raises them to `error` in its root `.dmtlint.yaml`
(`global.linters-settings.rbac.rules.<rule>.impact`) once its modules are clean. Per-rule levels are
read from the root only, as for every dmt linter, and a module cannot lower them. A rule the root
leaves unset falls back to the linter's `impact` below `warn`: `impact: ignored` on `rbac` in a module's
`.dmtlint.yaml` silences it there. A level other than `ignored`, `warn`, `error` or `critical` is a
configuration error.

**Limits worth knowing:**

- A conditional rule is checked only where it renders: with the default values, `dmt lint --values-file` or `dmt lint --matrix`.
- **The three states a module can be in when the new `dmt` first runs.** *Only the legacy scheme* (an external module not yet migrated): `contract` reports one "migrate" finding per object; with an `rbac.yaml`, `sync` reports the generated files as absent and names the cause -- the template renders the legacy scheme -- and `--fix` leaves the legacy file alone (no generator header) with the generated version beside it. *Only the 1.78 scheme*: the ordinary case described above. *Both schemes behind the version gate* (`rbacv2-migrate-module.sh` without `--replace`): the linter's values answer the gate with the 1.78 model, so `contract` and `sync` see exactly the new objects and the legacy branch is neither judged nor "extra"; `--fix` never rewrites a gated file -- regenerating it would drop the legacy branch -- and says so. The legacy branch itself is exercised with `dmt lint --values-file` setting `global.deckhouseVersion` below 1.78; `--matrix` varies module values only, not the platform version.
- The declaration is one per module and describes the union of editions. Linting a single edition directory shows the edition-only objects as absent; lint the merged tree as CI does. An `rbac.yaml` inside an edition overlay (`ee/be/modules`, `ee/se-plus/modules`, ..., and `ee/modules` for a module that also exists in `modules/`) is an error: CI merges the overlays over `modules/` before linting, so a copy there would shadow the base one or go unseen. A module that has no base elsewhere -- EE-only in `ee/modules/<module>`, or living in one edition directory only such as `ee/be/modules/350-node-local-dns` -- has its base there.
- The generator refuses what the declaration alone cannot know is wrong: system levels on a module whose `module.yaml` names no `subsystems` (set `subsystems` in `rbac.yaml`); an account in a component directory named unlike it (the placement rule wants `<dir>` or `<module>-<dir>`) or in a nested directory; a capability marker past 63 characters (a module name of 32 characters and up with a `superadmin` level).
- Built-in Kubernetes resources (`""`/configmaps, `apps`/deployments, `rbac.authorization.k8s.io`/clusterroles, ...) need no `scope`: the validator knows them. Anything else without a CRD in the module declares its scope.
- An `rbac.yaml` of the earlier, never consumed shape (no `apiVersion`) is named for what it is: delete it and run `--fix` to write the declaration from the render.
- Under `--matrix` the first declaration is written from the union of every variant's render; objects rendered only under values other than the defaults are still invisible to a default run, so lint with `--values-file` before the first regeneration if the module has such templates.
- `impact: ignored` on a rule switches its autofix off with it: `--fix` never rewrites files on behalf of findings nobody sees.
- A `when` condition must parse as a Helm expression (sprig and Helm functions are known); whether it holds under the linter's value stubs is decided by the render -- a condition that breaks the render is reported by the `helm-render` rule, and the module is not linted further.
- `dmt lint remote` does not run these rules: a published image carries no chart to render.
- The keys of the `rbac` configuration blocks are checked down to a rule's `impact` and the `kind`/`name` of an exclusion entry: every unknown key is reported in one error, not dropped in silence.

