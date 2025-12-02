# Container Linter

Checks containers inside the template spec. This linter protects against the next cases:

- containers with the duplicated names
- containers with the duplicated env variables
- misconfigured images repository and digest
- imagePullPolicy is "Always" (should be unspecified or "IfNotPresent")
- ephemeral storage is not defined in .resources
- SecurityContext is not defined
- ReadOnlyRootFilesystem is not set to true (prevents write access to container root filesystem)
- AllowPrivilegeEscalation is not set to false (prevents privilege escalation attacks)
- Seccomp profile is not properly configured (ensures default seccomp filtering is enabled)
- container uses port <= 1024
- Checks for probes defined in containers.
## Overview

The **Container Linter** validates Kubernetes objects and their container specifications to ensure compliance with security best practices, resource management, and operational standards. This linter examines Deployments, DaemonSets, StatefulSets, Pods, Jobs, and CronJobs to enforce consistent configuration across all workloads.

Proper container configuration is critical for cluster stability, security, and resource efficiency. The linter helps prevent common misconfigurations that can lead to security vulnerabilities, resource exhaustion, and operational issues in production environments.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [object-recommended-labels](#object-recommended-labels) | Validates required labels (module, heritage) | ❌ | enabled |
| [object-namespace-labels](#object-namespace-labels) | Validates Prometheus watcher label on d8-* namespaces | ❌ | enabled |
| [object-api-version](#object-api-version) | Validates API versions are not deprecated | ❌ | enabled |
| [object-priority-class](#object-priority-class) | Validates PriorityClass is set and allowed | ❌ | enabled |
| [dns-policy](#dns-policy) | Validates DNS policy for hostNetwork pods | ✅ | enabled |
| [controller-security-context](#controller-security-context) | Validates Pod-level security context | ✅ | enabled |
| [object-revision-history-limit](#object-revision-history-limit) | Validates Deployment revision history limit ≤ 2 | ❌ | enabled |
| [name-duplicates](#name-duplicates) | Validates no duplicate container names | ❌ | enabled |
| [read-only-root-filesystem](#read-only-root-filesystem) | Validates containers use read-only root filesystem | ✅ | enabled |
| [host-network-ports](#host-network-ports) | Validates host network and host port usage | ✅ | enabled |
| [env-variables-duplicates](#env-variables-duplicates) | Validates no duplicate environment variables | ❌ | enabled |
| [image-digest](#image-digest) | Validates image registry compliance | ✅ | enabled |
| [image-pull-policy](#image-pull-policy) | Validates imagePullPolicy is correct | ❌ | enabled |
| [resources](#resources) | Validates ephemeral storage is defined | ✅ | enabled |
| [security-context](#security-context) | Validates container-level security context | ✅ | enabled |
| [ports](#ports) | Validates container ports > 1024 | ✅ | enabled |
| [liveness-probe](#liveness-probe) | Validates liveness probe configuration | ✅ | enabled |
| [readiness-probe](#readiness-probe) | Validates readiness probe configuration | ✅ | enabled |
| [no-new-privileges](#no-new-privileges) | Validates containers don't allow privilege escalation | ✅ | enabled |
| [seccomp-profile](#seccomp-profile) | Validates seccomp profile configuration | ✅ | enabled |

## Rule Details

### object-recommended-labels

**Purpose:** Ensures all Kubernetes objects have required labels for proper identification, monitoring, and management within the Deckhouse platform. These labels are used for resource tracking, metrics collection, and operational automation.

**Description:**

Every Kubernetes object must have two mandatory labels:
- `module` - Identifies which Deckhouse module owns the resource
- `heritage` - Indicates the resource is managed by Deckhouse

**What it checks:**

1. Validates presence of `module` label on all objects
2. Validates presence of `heritage` label on all objects

**Why it matters:**

These labels are essential for Deckhouse's resource management system. Without them, resources can't be properly tracked, monitored metrics may be missing, and automated cleanup or upgrades may fail.

**Examples:**

❌ **Incorrect** - Missing labels:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: d8-my-module
  labels:
    app: my-app
    # Missing: module and heritage labels
```

**Error:**
```
Object does not have the label "module"
Object does not have the label "heritage"
```

✅ **Correct** - All required labels present:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: d8-my-module
  labels:
    app: my-app
    module: my-module
    heritage: deckhouse
```

---

### object-namespace-labels

**Purpose:** Ensures Deckhouse namespaces (prefixed with `d8-`) have the Prometheus rules watcher label enabled. This allows Prometheus to automatically discover and apply monitoring rules for the namespace.

**Description:**

Namespaces starting with `d8-` with `PrometheusRule` must have the label `prometheus.deckhouse.io/rules-watcher-enabled: "true"` to enable automatic Prometheus rules discovery.

**What it checks:**

1. Identifies Namespace objects with names starting with `d8-`
2. Find kind `PrometheusRule` with the same Namespace
3. Validates the `prometheus.deckhouse.io/rules-watcher-enabled` label is set to `"true"`

**Why it matters:**

Without this label, Prometheus won't monitor the namespace properly, leading to missing metrics, alerts, and observability gaps for module workloads.

**Examples:**

❌ **Incorrect** - Missing Prometheus label:

```yaml
apiVersion: v1
kind: PrometheusRule
metadata:
  ...
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-my-module
  labels:
    module: my-module
    # Missing: prometheus.deckhouse.io/rules-watcher-enabled
```

**Error:**
```
Namespace object does not have the label "prometheus.deckhouse.io/rules-watcher-enabled"
```

✅ **Correct** - Prometheus label present:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  ...
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-my-module
  labels:
    module: my-module
    heritage: deckhouse
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
```

✅ **Correct** - No PrometheusRule:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: d8-my-module
  labels:
    module: my-module
    heritage: deckhouse
```

---

### object-api-version

**Purpose:** Prevents use of deprecated Kubernetes API versions that may be removed in future Kubernetes releases. This ensures module compatibility with current and future Kubernetes versions.

**Description:**

Validates that objects use current, stable API versions instead of deprecated or beta versions.

**What it checks:**

Enforces correct API versions for common resource types:

| Resource Type | Required API Version |
|---------------|---------------------|
| Role, RoleBinding, ClusterRole, ClusterRoleBinding | `rbac.authorization.k8s.io/v1` |
| Deployment, DaemonSet, StatefulSet | `apps/v1` |
| Ingress | `networking.k8s.io/v1` |
| PriorityClass | `scheduling.k8s.io/v1` |
| PodSecurityPolicy | `policy/v1beta1` |
| NetworkPolicy | `networking.k8s.io/v1` |

**Why it matters:**

Deprecated API versions will be removed in future Kubernetes releases, causing module deployment failures. Using current API versions ensures long-term compatibility.

**Examples:**

❌ **Incorrect** - Deprecated API version:

```yaml
apiVersion: extensions/v1beta1  # Deprecated
kind: Deployment
metadata:
  name: my-app
```

**Error:**
```
Object defined using deprecated api version, wanted "apps/v1"
```

✅ **Correct** - Current API version:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
```

---

### object-priority-class

**Purpose:** Ensures all workloads have appropriate priority classes assigned for proper pod scheduling and preemption behavior. This prevents workloads from running without priority, which can cause scheduling issues.

**Description:**

All Deployment, DaemonSet, and StatefulSet objects must specify a valid PriorityClass from the allowed list.

**What it checks:**

1. Validates `spec.template.spec.priorityClassName` is not empty
2. Validates the priority class is from the allowed list

**Allowed PriorityClasses:**
- `system-node-critical` - Critical system components
- `system-cluster-critical` - Critical cluster services
- `cluster-critical` - Critical cluster workloads
- `cluster-medium` - Medium priority cluster workloads
- `cluster-low` - Low priority cluster workloads
- `production-high` - High priority production workloads
- `production-medium` - Medium priority production workloads
- `production-low` - Low priority production workloads
- `staging` - Staging environment workloads
- `develop` - Development environment workloads
- `standby` - Standby workloads

**Why it matters:**

Priority classes control pod scheduling and preemption. Without proper priorities, critical workloads may be evicted, or low-priority workloads may consume resources needed by important services.

**Examples:**

❌ **Incorrect** - Missing priority class:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      # Missing: priorityClassName
      containers:
        - name: app
          image: my-image
```

**Error:**
```
Priority class must not be empty
```

❌ **Incorrect** - Invalid priority class:

```yaml
spec:
  template:
    spec:
      priorityClassName: custom-priority  # Not in allowed list
```

**Error:**
```
Priority class is not allowed
```

✅ **Correct** - Valid priority class:

```yaml
spec:
  template:
    spec:
      priorityClassName: production-medium
      containers:
        - name: app
          image: my-image
```

---

### dns-policy

**Purpose:** Ensures pods using `hostNetwork: true` have the correct DNS policy to access cluster DNS services. Without this, pods on the host network can't resolve cluster service names.

**Description:**

When a pod uses `hostNetwork: true`, it must set `dnsPolicy: ClusterFirstWithHostNet` to enable cluster DNS resolution while using the host network.

**What it checks:**

1. Detects pods with `hostNetwork: true`
2. Validates `dnsPolicy` is set to `ClusterFirstWithHostNet`

**Why it matters:**

Pods with host networking use the node's DNS by default, which can't resolve Kubernetes service names. This breaks service discovery and inter-service communication.

**Examples:**

❌ **Incorrect** - Wrong DNS policy with host network:

```yaml
spec:
  template:
    spec:
      hostNetwork: true
      dnsPolicy: ClusterFirst  # Wrong for hostNetwork
      containers:
        - name: app
          image: my-image
```

**Error:**
```
dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`
```

✅ **Correct** - Proper DNS policy:

```yaml
spec:
  template:
    spec:
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      containers:
        - name: app
          image: my-image
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      dns-policy:
        - kind: DaemonSet
          name: special-networking-component
```

---

### controller-security-context

**Purpose:** Ensures pod-level security context is properly configured with required UID/GID settings and non-root execution. This establishes baseline security for all containers in the pod.

**Description:**

All pod specifications must define a security context with `runAsUser`, `runAsGroup`, and `runAsNonRoot` parameters. These settings must follow specific UID/GID combinations.

**What it checks:**

1. Pod security context is defined
2. `runAsNonRoot`, `runAsUser`, and `runAsGroup` are all specified
3. For `runAsNonRoot: true`: UID:GID must be `65534:65534` (nobody) or `64535:64535` (deckhouse)
4. For `runAsNonRoot: false`: UID:GID must be `0:0` (root)

**Why it matters:**

Running containers as root is a security risk. Properly configured security contexts enforce least-privilege principles and prevent privilege escalation attacks.

**Examples:**

❌ **Incorrect** - Missing security context:

```yaml
spec:
  template:
    spec:
      # Missing: securityContext
      containers:
        - name: app
          image: my-image
```

**Error:**
```
Object's SecurityContext is not defined
```

❌ **Incorrect** - Invalid UID/GID combination:

```yaml
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
```

**Error:**
```
Object's SecurityContext has `runAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)
```

✅ **Correct** - Non-root with nobody user:

```yaml
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        runAsGroup: 65534
      containers:
        - name: app
          image: my-image
```

✅ **Correct** - Non-root with deckhouse user:

```yaml
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 64535
        runAsGroup: 64535
      containers:
        - name: app
          image: my-image
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      controller-security-context:
        - kind: DaemonSet
          name: privileged-system-component
```

---

### object-revision-history-limit

**Purpose:** Limits the number of old ReplicaSets retained for each Deployment to reduce control plane resource consumption. Deckhouse doesn't use rollback functionality, so keeping many old ReplicaSets is wasteful.

**Description:**

Deployment objects must set `spec.revisionHistoryLimit` to a value ≤ 2.

**What it checks:**

1. `spec.revisionHistoryLimit` is specified
2. Value is ≤ 2

**Why it matters:**

Each ReplicaSet consumes etcd storage and increases API server load. Limiting revision history reduces control plane pressure while keeping enough history for manual comparison if needed.

**Examples:**

❌ **Incorrect** - Missing or too high:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  # Missing: revisionHistoryLimit (defaults to 10)
  template:
    spec:
      containers:
        - name: app
          image: my-image
```

**Error:**
```
Deployment spec.revisionHistoryLimit must be less or equal to 2
```

❌ **Incorrect** - Value too high:

```yaml
spec:
  revisionHistoryLimit: 10
```

**Error:**
```
Deployment spec.revisionHistoryLimit must be less or equal to 2
```

✅ **Correct** - Appropriate limit:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  revisionHistoryLimit: 2
  template:
    spec:
      containers:
        - name: app
          image: my-image
```

---

### name-duplicates

**Purpose:** Prevents duplicate container names within a pod, which would cause Kubernetes to reject the pod specification.

**Description:**

All containers in a pod (including init containers and ephemeral containers) must have unique names.

**What it checks:**

1. Scans all containers in a pod specification
2. Detects duplicate container names

**Why it matters:**

Kubernetes requires unique container names within a pod. Duplicates cause pod creation failures and deployment issues.

**Examples:**

❌ **Incorrect** - Duplicate names:

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          image: my-image:v1
        - name: app  # Duplicate
          image: sidecar:v1
```

**Error:**
```
Duplicate container name
```

✅ **Correct** - Unique names:

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          image: my-image:v1
        - name: sidecar
          image: sidecar:v1
```

---

### read-only-root-filesystem

**Purpose:** Enforces immutable container filesystems as a security best practice. This prevents malicious processes from modifying the container's filesystem.

**Description:**

All containers must set `securityContext.readOnlyRootFilesystem: true`. Containers needing writable directories should use emptyDir or other volume mounts.

**What it checks:**

1. Container security context is defined
2. `readOnlyRootFilesystem` parameter is specified
3. Value is `true`

**Why it matters:**

Read-only root filesystems prevent unauthorized modifications, malware persistence, and privilege escalation attacks. This is a fundamental security hardening practice.

⚠️ **Important** 

When using conditions in templates, they must be rendered under all conditions.

For example:
```yaml
{{- if .Values.csiNfs.v3support }}
```

Only works if:
- csiNfs exists in .Values.
- There is v3support inside it.

If csiNfs is missing, there will be a rendering error (nil pointer evaluating).

Therefore, no linter check will occur under it.

In the case of
```yaml
{{- if (default false (dig “csiNfs” “v3support” .Values)) }}
```
- dig returns nil if the key does not exist, and there will be no error.
- default false will substitute false for nil.

The result is safer: if the key does not exist, the condition will not work, but the template will not fail.

Linter will be able to check the block under the condition.

**Examples:**

❌ **Incorrect** - Missing or false:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      readOnlyRootFilesystem: false
```

**Error:**
```
Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`
```

✅ **Correct** - Read-only filesystem:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      readOnlyRootFilesystem: true
    volumeMounts:
      - name: tmp
        mountPath: /tmp
volumes:
  - name: tmp
    emptyDir: {}
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      read-only-root-filesystem:
        - kind: Deployment
          name: deckhouse
          container: init-downloaded-modules
```

---

### host-network-ports

**Purpose:** Ensures pods using host networking or host ports only use the designated port range [4200-4299]. This prevents conflicts with system services and other applications.

**Description:**

Validates port usage for:
- Pods with `hostNetwork: true` - container ports must be in range [4200-4299]
- Containers using `hostPort` - host ports must be in range [4200-4299]

**What it checks:**

1. Detects pods with `hostNetwork: true`
2. Validates container ports are in allowed range
3. Validates host ports are in allowed range

**Why it matters:**

Using host networking or host ports outside the designated range can conflict with system services (SSH, kubelet, etc.) or other applications, causing port conflicts and service failures.

**Examples:**

❌ **Incorrect** - Port outside allowed range:

```yaml
spec:
  template:
    spec:
      hostNetwork: true
      containers:
        - name: app
          image: my-image
          ports:
            - containerPort: 8080  # Outside [4200-4299]
```

**Error:**
```
Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]
```

✅ **Correct** - Port in allowed range:

```yaml
spec:
  template:
    spec:
      hostNetwork: true
      containers:
        - name: app
          image: my-image
          ports:
            - containerPort: 4250  # Within [4200-4299]
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      host-network-ports:
        - kind: DaemonSet
          name: network-agent
          container: agent
```

---

### env-variables-duplicates

**Purpose:** Prevents duplicate environment variable names within a container, which causes undefined behavior and configuration errors.

**Description:**

All environment variables within a container must have unique names.

**What it checks:**

1. Scans all environment variables in each container
2. Detects duplicate variable names

**Why it matters:**

Duplicate environment variables lead to unpredictable behavior as only one value will be used, and it's unclear which one. This causes configuration bugs and debugging difficulties.

**Examples:**

❌ **Incorrect** - Duplicate env vars:

```yaml
containers:
  - name: app
    image: my-image
    env:
      - name: LOG_LEVEL
        value: "info"
      - name: LOG_LEVEL  # Duplicate
        value: "debug"
```

**Error:**
```
Container has two env variables with same name
```

✅ **Correct** - Unique env vars:

```yaml
containers:
  - name: app
    image: my-image
    env:
      - name: LOG_LEVEL
        value: "info"
      - name: DEBUG_MODE
        value: "false"
```

---

### image-digest

**Purpose:** Ensures all container images are pulled from the designated Deckhouse registry. This maintains supply chain security and ensures image availability.

**Description:**

All container images must be served from the default registry: `registry.example.com/deckhouse`

**What it checks:**

1. Parses image repository from container image specifications
2. Validates repository matches the default registry

**Why it matters:**

Using external registries bypasses Deckhouse's image verification, mirroring, and caching mechanisms. This can lead to supply chain attacks, missing images, or rate limiting issues with public registries.

**Examples:**

❌ **Incorrect** - External registry:

```yaml
containers:
  - name: app
    image: docker.io/library/nginx:latest
```

**Error:**
```
All images must be deployed from the same default registry: registry.example.com/deckhouse current: docker.io/library
```

✅ **Correct** - Deckhouse registry:

```yaml
containers:
  - name: app
    image: registry.example.com/deckhouse/nginx:v1.25.3
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      image-digest:
        - kind: Deployment
          name: third-party-component
          container: agent
```

---

### image-pull-policy

**Purpose:** Ensures correct image pull policy settings to optimize image pulling behavior and prevent unnecessary registry traffic.

**Description:**

- For the main Deckhouse deployment: `imagePullPolicy` should be unspecified or `Always`
- For all other workloads: `imagePullPolicy` should be unspecified or `IfNotPresent`

**What it checks:**

1. Special handling for `d8-system/deckhouse` Deployment (requires `Always`)
2. All other containers must use unspecified or `IfNotPresent`

**Why it matters:**

The main Deckhouse controller needs `Always` to ensure it gets the latest version. Other workloads should use `IfNotPresent` to reduce registry load and improve pod startup time.

**Examples:**

❌ **Incorrect** - Wrong policy for regular workload:

```yaml
containers:
  - name: app
    image: my-image:v1.0.0
    imagePullPolicy: Always  # Wrong for non-deckhouse workloads
```

**Error:**
```
Container imagePullPolicy should be unspecified or "IfNotPresent"
```

✅ **Correct** - Proper policy:

```yaml
containers:
  - name: app
    image: my-image:v1.0.0
    imagePullPolicy: IfNotPresent
```

Or simply omit it:

```yaml
containers:
  - name: app
    image: my-image:v1.0.0
    # imagePullPolicy defaults to IfNotPresent
```

---

### resources

**Purpose:** Ensures all containers define ephemeral storage requests to prevent uncontrolled disk usage that could fill up node storage and cause node failures.

**Description:**

All containers must specify `resources.requests.ephemeral-storage`.

**What it checks:**

1. Validates `resources.requests.ephemeral-storage` is defined
2. Validates value is greater than 0

**Why it matters:**

Without ephemeral storage limits, containers can fill up node disk space, causing node failures, pod evictions, and cluster instability. Defining requests enables proper scheduling and resource management.

**Examples:**

❌ **Incorrect** - Missing ephemeral storage:

```yaml
containers:
  - name: app
    image: my-image
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
        # Missing: ephemeral-storage
```

**Error:**
```
Ephemeral storage for container is not defined in Resources.Requests
```

✅ **Correct** - Ephemeral storage defined:

```yaml
containers:
  - name: app
    image: my-image
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
        ephemeral-storage: "1Gi"
      limits:
        memory: "256Mi"
        cpu: "200m"
        ephemeral-storage: "2Gi"
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      resources:
        - kind: Deployment
          name: standby-holder
          container: reserve-resources
```

---

### security-context

**Purpose:** Ensures all containers have a security context defined, which is required for container-level security settings like read-only filesystem, capabilities, and privilege management.

**Description:**

All containers must have a `securityContext` defined at the container level (in addition to the pod-level security context).

**What it checks:**

1. Validates `securityContext` is present in container specification

**Why it matters:**

Container security context allows fine-grained security controls per container. Without it, security settings like read-only filesystem, dropped capabilities, and privilege restrictions can't be applied.

**Examples:**

❌ **Incorrect** - Missing container security context:

```yaml
containers:
  - name: app
    image: my-image
    # Missing: securityContext
```

**Error:**
```
Container ContainerSecurityContext is not defined
```

✅ **Correct** - Security context defined:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      security-context:
        - kind: Deployment
          name: legacy-app
          container: app
```

---

### ports

**Purpose:** Prevents containers from using privileged ports (≤1024), which require root privileges and violate least-privilege security principles.

**Description:**

All container ports must be greater than 1024.

**What it checks:**

1. Examines all container port definitions
2. Validates `containerPort` > 1024

**Why it matters:**

Ports ≤1024 are privileged and require root access. Running containers with privileged ports increases security risks and violates the principle of running containers as non-root users.

**Examples:**

❌ **Incorrect** - Privileged port:

```yaml
containers:
  - name: app
    image: my-image
    ports:
      - containerPort: 80  # Privileged port
        protocol: TCP
```

**Error:**
```
Container uses port <= 1024
```

✅ **Correct** - Non-privileged port:

```yaml
containers:
  - name: app
    image: my-image
    ports:
      - containerPort: 8080  # Non-privileged port
        protocol: TCP
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      ports:
        - kind: DaemonSet
          name: special-network-component
          container: agent
```

---

### liveness-probe

**Purpose:** Ensures containers have liveness probes configured to detect and recover from application deadlocks and failures. Without liveness probes, Kubernetes can't detect or restart failed containers.

**Description:**

All regular containers (not init containers) in Deployment, DaemonSet, and StatefulSet must define a liveness probe with exactly one handler type.

**What it checks:**

1. Validates `livenessProbe` is defined
2. Validates exactly one probe handler is configured (Exec, HTTP, TCP, or GRPC)

**Why it matters:**

Liveness probes enable Kubernetes to detect when containers are stuck or deadlocked and automatically restart them. Without liveness probes, unhealthy containers continue running, causing service degradation.

**Examples:**

❌ **Incorrect** - Missing liveness probe:

```yaml
containers:
  - name: app
    image: my-image
    # Missing: livenessProbe
```

**Error:**
```
Container does not contain liveness-probe
```

❌ **Incorrect** - Invalid probe configuration:

```yaml
containers:
  - name: app
    image: my-image
    livenessProbe:
      # No handler defined
      initialDelaySeconds: 30
```

**Error:**
```
Container does not use correct liveness-probe
```

✅ **Correct** - HTTP liveness probe:

```yaml
containers:
  - name: app
    image: my-image
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
```

✅ **Correct** - TCP liveness probe:

```yaml
containers:
  - name: app
    image: my-image
    livenessProbe:
      tcpSocket:
        port: 8080
      initialDelaySeconds: 15
      periodSeconds: 20
```

✅ **Correct** - Exec liveness probe:

```yaml
containers:
  - name: app
    image: my-image
    livenessProbe:
      exec:
        command:
          - /bin/sh
          - -c
          - ps aux | grep my-process
      initialDelaySeconds: 30
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      liveness-probe:
        - kind: Deployment
          name: standby-holder
          container: reserve-resources
```

---

### readiness-probe

**Purpose:** Ensures containers have readiness probes configured to signal when they're ready to accept traffic. Without readiness probes, Kubernetes may route traffic to containers that aren't ready, causing failed requests.

**Description:**

All regular containers (not init containers) in Deployment, DaemonSet, and StatefulSet must define a readiness probe with exactly one handler type.

**What it checks:**

1. Validates `readinessProbe` is defined
2. Validates exactly one probe handler is configured (Exec, HTTP, TCP, or GRPC)

**Why it matters:**

Readiness probes control when containers receive traffic from Services. Without them, new pods receive traffic before they're ready, and degraded pods continue receiving traffic, causing user-facing errors.

**Examples:**

❌ **Incorrect** - Missing readiness probe:

```yaml
containers:
  - name: app
    image: my-image
    # Missing: readinessProbe
```

**Error:**
```
Container does not contain readiness-probe
```

❌ **Incorrect** - Invalid probe configuration:

```yaml
containers:
  - name: app
    image: my-image
    readinessProbe:
      # No handler defined
      initialDelaySeconds: 10
```

**Error:**
```
Container does not use correct readiness-probe
```

✅ **Correct** - HTTP readiness probe:

```yaml
containers:
  - name: app
    image: my-image
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 5
```

✅ **Correct** - TCP readiness probe:

```yaml
containers:
  - name: app
    image: my-image
    readinessProbe:
      tcpSocket:
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 10
```

✅ **Correct** - GRPC readiness probe:

```yaml
containers:
  - name: app
    image: my-image
    readinessProbe:
      grpc:
        port: 9090
      initialDelaySeconds: 5
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      readiness-probe:
        - kind: Deployment
          name: standby-holder
          container: reserve-resources
```

### no-new-privileges

**Purpose:** Ensures containers don't allow privilege escalation by setting `allowPrivilegeEscalation` to `false`. This prevents processes from gaining additional privileges beyond what the container starts with.

**Description:**

Validates that container security contexts explicitly set `allowPrivilegeEscalation: false` or leave it unset (defaults to `false` in most Kubernetes versions).

**What it checks:**

1. Validates `allowPrivilegeEscalation` is set to `false` in container security context
2. Warns if `allowPrivilegeEscalation` is set to `true`

**Why it matters:**

Privilege escalation allows processes to gain additional privileges, potentially allowing attackers to break out of container isolation. Setting `allowPrivilegeEscalation: false` provides an additional security layer.

**Examples:**

❌ **Incorrect** - Privilege escalation allowed:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      allowPrivilegeEscalation: true  # ❌ Allows privilege escalation
```

**Error:**
```
Container allows privilege escalation (allowPrivilegeEscalation is true)
```

✅ **Correct** - Privilege escalation disabled:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      allowPrivilegeEscalation: false  # ✅ Explicitly disabled
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      no-new-privileges:
        - kind: Deployment
          name: privileged-deployment
          container: init-container
```

### seccomp-profile

**Purpose:** Ensures containers use appropriate seccomp profiles for system call filtering. Seccomp provides an additional security layer by restricting which system calls processes can make.

**Description:**

Validates that containers have proper seccomp profile configuration. The recommended setting is `seccompProfile.type: RuntimeDefault` which uses the container runtime's default seccomp profile.

**What it checks:**

1. Validates seccomp profile is configured (either at pod or container level)
2. Recommends `RuntimeDefault` profile for security
3. Warns against `Unconfined` profile which disables seccomp filtering
4. Validates custom profiles have proper configuration

**Why it matters:**

Seccomp filtering reduces the attack surface by restricting system calls. Without proper seccomp configuration, containers have access to the full system call interface, increasing security risks.

**Examples:**

❌ **Incorrect** - No seccomp profile:

```yaml
containers:
  - name: app
    image: my-image
    # Missing: seccompProfile configuration
```

**Warning:**
```
No seccomp profile specified - consider explicitly setting seccompProfile.type to 'RuntimeDefault'
```

❌ **Incorrect** - Seccomp disabled:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      seccompProfile:
        type: Unconfined  # ❌ Disables seccomp filtering
```

**Error:**
```
Container has seccompProfile.type set to 'Unconfined' which disables seccomp filtering and poses security risks - use 'RuntimeDefault' instead
```

✅ **Correct** - Runtime default profile:

```yaml
containers:
  - name: app
    image: my-image
    securityContext:
      seccompProfile:
        type: RuntimeDefault  # ✅ Uses secure default profile
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      seccomp-profile:
        - kind: DaemonSet
          name: system-daemon
          container: system-container
```

## Configuration

The Container linter can be configured at both the module level and for individual rules.

### Module-Level Settings

Configure the overall impact level for the container linter:

```yaml
# .dmt.yaml
linters-settings:
  container:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### Exclude Rules

Many rules support excluding specific objects or containers:

```yaml
# .dmt.yaml
linters-settings:
  container:
    exclude-rules:
      # Exclude by kind and object name
      dns-policy:
        - kind: Deployment
          name: machine-controller-manager
      
      # Exclude by kind, object name, and container name
      read-only-root-filesystem:
        - kind: Deployment
          name: deckhouse
          container: init-downloaded-modules
      
      # Exclude multiple containers
      security-context:
        - kind: Deployment
          name: caps-controller-manager
          container: caps-controller-manager
        - kind: Deployment
          name: standby-holder-name
      
      # Exclude for specific container in specific object
      liveness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      readiness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      resources:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      image-digest:
        - kind: Deployment
          name: okmeter
          container: okagent
      
      host-network-ports:
        - kind: DaemonSet
          name: network-plugin
          container: agent
      
      ports:
        - kind: DaemonSet
          name: system-component
          container: main
```

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  container:
    # Global impact level for all container rules
    impact: error
    # Exclude specific objects/containers from rules
    exclude-rules:
      read-only-root-filesystem:
        - kind: Deployment
          name: deckhouse
          container: init-downloaded-modules
      # exclude if object kind, object name and containers name are equal
      no-new-privileges:
        - kind: Deployment
          name: privileged-deployment
          container: init-container
      # exclude if object kind, object name and containers name are equal
      seccomp-profile:
        - kind: DaemonSet
          name: system-daemon
          container: system-container
      # exclude if object kind, object name and containers name are equal
      
      resources:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      security-context:
        - kind: Deployment
          name: caps-controller-manager
          container: caps-controller-manager
      
      dns-policy:
        - kind: Deployment
          name: machine-controller-manager
      
      liveness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      readiness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      
      image-digest:
        - kind: Deployment
          name: okmeter
          container: okagent
    impact: error
```
```

### Configuration in Module Directory

You can also place a `.dmt.yaml` configuration file directly in your module directory:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  container:
    impact: warning  # More lenient for this specific module
    exclude-rules:
      read-only-root-filesystem:
        - kind: Deployment
          name: legacy-app
          container: main
```

## Common Issues

### Issue: Missing required labels

**Symptom:**
```
Error: Object does not have the label "module"
Error: Object does not have the label "heritage"
```

**Cause:** Kubernetes objects don't have the required `module` and `heritage` labels.

**Solutions:**

1. **Add labels to all objects:**

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: my-app
     labels:
       module: my-module
       heritage: deckhouse
   ```

2. **Use Helm templates to ensure consistency:**

   ```yaml
   # templates/_helpers.tpl
   {{- define "module.labels" -}}
   module: {{ .Chart.Name }}
   heritage: deckhouse
   {{- end -}}
   
   # templates/deployment.yaml
   metadata:
     labels:
       {{- include "module.labels" . | nindent 6 }}
   ```

### Issue: Deprecated API version

**Symptom:**
```
Error: Object defined using deprecated api version, wanted "apps/v1"
```

**Cause:** Using old or deprecated Kubernetes API versions.

**Solutions:**

1. **Update to current API version:**

   ```bash
   # Update all Deployments
   sed -i 's|apiVersion: extensions/v1beta1|apiVersion: apps/v1|g' templates/*.yaml
   ```

2. **Check Kubernetes API migration guides:**
   - Refer to [Kubernetes API deprecation guide](https://kubernetes.io/docs/reference/using-api/deprecation-guide/)

### Issue: Security context violations

**Symptom:**
```
Error: Container's SecurityContext has `ReadOnlyRootFilesystem: false`, but it must be `true`
Error: Container's SecurityContext missing parameter ReadOnlyRootFilesystem
```

**Cause:** Containers don't have proper security context configuration.

**Solutions:**

1. **Add complete security context:**

   ```yaml
   spec:
     template:
       spec:
         securityContext:
           runAsNonRoot: true
           runAsUser: 65534
           runAsGroup: 65534
         containers:
           - name: app
             securityContext:
               readOnlyRootFilesystem: true
               allowPrivilegeEscalation: false
               capabilities:
                 drop:
                   - ALL
             volumeMounts:
               - name: tmp
                 mountPath: /tmp
       volumes:
         - name: tmp
           emptyDir: {}
   ```

2. **For containers that need writable directories:**

   Use volume mounts for writable paths:
   ```yaml
   volumeMounts:
     - name: tmp
       mountPath: /tmp
     - name: cache
       mountPath: /var/cache
     - name: logs
       mountPath: /var/log
   volumes:
     - name: tmp
       emptyDir: {}
     - name: cache
       emptyDir: {}
     - name: logs
       emptyDir: {}
   ```

### Issue: Missing probes

**Symptom:**
```
Error: Container does not contain liveness-probe
Error: Container does not contain readiness-probe
```

**Cause:** Containers don't have health check probes configured.

**Solutions:**

1. **Add HTTP probes (recommended for web applications):**

   ```yaml
   containers:
     - name: app
       livenessProbe:
         httpGet:
           path: /healthz
           port: 8080
         initialDelaySeconds: 30
         periodSeconds: 10
       readinessProbe:
         httpGet:
           path: /ready
           port: 8080
         initialDelaySeconds: 10
         periodSeconds: 5
   ```

2. **Add TCP probes (for non-HTTP services):**

   ```yaml
   containers:
     - name: app
       livenessProbe:
         tcpSocket:
           port: 8080
         initialDelaySeconds: 15
       readinessProbe:
         tcpSocket:
           port: 8080
         initialDelaySeconds: 5
   ```

3. **Add exec probes (for custom health checks):**

   ```yaml
   containers:
     - name: app
       livenessProbe:
         exec:
           command:
             - /bin/sh
             - -c
             - /healthcheck.sh
         initialDelaySeconds: 30
       readinessProbe:
         exec:
           command:
             - /bin/sh
             - -c
             - /readycheck.sh
         initialDelaySeconds: 10
   ```

### Issue: Missing ephemeral storage

**Symptom:**
```
Error: Ephemeral storage for container is not defined in Resources.Requests
```

**Cause:** Container resource requests don't include ephemeral storage.

**Solutions:**

1. **Add ephemeral storage to resource requests:**

   ```yaml
   containers:
     - name: app
       resources:
         requests:
           memory: "128Mi"
           cpu: "100m"
           ephemeral-storage: "1Gi"
         limits:
           memory: "256Mi"
           cpu: "200m"
           ephemeral-storage: "2Gi"
   ```

2. **Calculate appropriate storage size:**

   Consider:
   - Application logs
   - Temporary files
   - Cache directories
   - Container image layers

### Issue: Invalid priority class

**Symptom:**
```
Error: Priority class must not be empty
Error: Priority class is not allowed
```

**Cause:** Priority class is missing or uses an invalid value.

**Solutions:**

1. **Add appropriate priority class:**

   ```yaml
   spec:
     template:
       spec:
         priorityClassName: production-medium
   ```

2. **Choose from allowed values based on criticality:**
   - Critical: `system-node-critical`, `system-cluster-critical`, `cluster-critical`
   - Production: `production-high`, `production-medium`, `production-low`
   - Non-production: `staging`, `develop`
   - Utility: `cluster-medium`, `cluster-low`, `standby`

### Issue: Port restrictions

**Symptom:**
```
Error: Container uses port <= 1024
Error: Pod running in hostNetwork and it's container port doesn't fit the range [4200,4299]
```

**Cause:** Using privileged ports or wrong ports with host networking.

**Solutions:**

1. **Use non-privileged ports:**

   ```yaml
   containers:
     - name: app
       ports:
         - containerPort: 8080  # Not 80
           protocol: TCP
   ```

2. **For host network, use designated range:**

   ```yaml
   spec:
     template:
       spec:
         hostNetwork: true
         dnsPolicy: ClusterFirstWithHostNet
         containers:
           - name: app
             ports:
               - containerPort: 4250  # Within [4200-4299]
   ```
