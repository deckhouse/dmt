# Templates Linter

## Overview

The **Templates Linter** validates Helm templates in Deckhouse modules to ensure they follow best practices for high availability, security, monitoring, and resource management. This linter checks pod controllers, services, monitoring configurations, and network resources to maintain consistent quality and operational reliability across all modules.

Proper template validation prevents runtime issues, ensures applications are production-ready with appropriate resource limits and disruption budgets, and maintains consistent monitoring and security practices throughout the platform.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [vpa](#vpa) | Validates VerticalPodAutoscalers for pod controllers | ✅ | enabled |
| [pdb](#pdb) | Validates PodDisruptionBudgets for deployments and statefulsets | ✅ | enabled |
| [kube-rbac-proxy](#kube-rbac-proxy) | Validates kube-rbac-proxy CA certificates in namespaces | ✅ | enabled |
| [service-port](#service-port) | Validates services use named target ports | ✅ | enabled |
| [ingress-rules](#ingress-rules) | Validates Ingress configuration snippets | ✅ | enabled |
| [prometheus-rules](#prometheus-rules) | Validates Prometheus rules with promtool and proper templates | ✅ | enabled |
| [grafana-dashboards](#grafana-dashboards) | Validates Grafana dashboard templates | ✅ | enabled |
| [cluster-domain](#cluster-domain) | Validates cluster domain configuration is dynamic | ❌ | enabled |
| [registry](#registry) | Validates registry secret configuration | ❌ | enabled |
| [werf](#werf) | Validates werf.yaml templates for git stage dependencies | ❌ | enabled |

## Rule Details

### vpa

**Purpose:** Ensures all pod controllers (Deployments, DaemonSets, StatefulSets) have corresponding VerticalPodAutoscalers (VPA) configured with proper resource policies. This enables automatic resource optimization and prevents resource exhaustion or waste.

**Description:**

Validates that every pod controller has a VPA targeting it, and that the VPA's container resource policies match the controller's containers. Each container must have proper min/max CPU and memory limits defined to allow VPA to make informed scaling decisions.

**What it checks:**

1. Every Deployment, DaemonSet, and StatefulSet has a corresponding VPA
2. VPA `targetRef` correctly references the controller (kind, name, namespace)
3. VPA `updateMode` cannot be "Auto"
4. VPA has `resourcePolicy.containerPolicies` for all containers (except when `updateMode: "Off"`)
5. Each container policy specifies:
   - `minAllowed.cpu` and `minAllowed.memory`
   - `maxAllowed.cpu` and `maxAllowed.memory`
6. Min values are less than max values
7. Container names in VPA match container names in the controller

**Why it matters:**

VPA ensures pods have appropriate resource requests based on actual usage, preventing:
- Over-provisioning that wastes cluster resources
- Under-provisioning that causes OOM kills and throttling
- Manual intervention for resource tuning

**Examples:**

❌ **Incorrect** - Deployment without VPA:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: app
          image: registry.example.com/my-app:v1.0.0
```

**Error:**
```
Error: No VPA is found for object
```

❌ **Incorrect** - VPA missing container policy:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  template:
    spec:
      containers:
        - name: app
          image: registry.example.com/my-app:v1.0.0
        - name: sidecar
          image: registry.example.com/sidecar:v1.0.0
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: Recreate
  resourcePolicy:
    containerPolicies:
      - containerName: app  # ❌ Missing sidecar container
        minAllowed:
          cpu: 10m
          memory: 50Mi
        maxAllowed:
          cpu: 100m
          memory: 200Mi
```

**Error:**
```
Error: The container should have corresponding VPA resourcePolicy entry
Object: Deployment/my-app ; container = sidecar
```

❌ **Incorrect** - VPA with invalid min/max:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: Recreate
  resourcePolicy:
    containerPolicies:
      - containerName: app
        minAllowed:
          cpu: 200m  # ❌ Min > Max
          memory: 50Mi
        maxAllowed:
          cpu: 100m
          memory: 200Mi
```

**Error:**
```
Error: MinAllowed.cpu for container app should be less than maxAllowed.cpu
```

❌ **Incorrect** - VPA with updateMode Auto:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: Auto  # ❌ updateMode cannot be "Auto"
  resourcePolicy:
    containerPolicies:
      - containerName: app
        minAllowed:
          cpu: 10m
          memory: 50Mi
        maxAllowed:
          cpu: 100m
          memory: 200Mi
```

**Error:**
```
Error: VPA updateMode cannot be 'Auto'
```

✅ **Correct** - Deployment with proper VPA:

```yaml
# templates/deployment.yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: app
          image: registry.example.com/my-app:v1.0.0
          resources:
            requests:
              cpu: 50m
              memory: 100Mi
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: Recreate
  resourcePolicy:
    containerPolicies:
      - containerName: app
        minAllowed:
          cpu: 10m
          memory: 50Mi
        maxAllowed:
          cpu: 500m
          memory: 500Mi
```

✅ **Correct** - Multiple containers with VPA:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: d8-my-module
spec:
  template:
    spec:
      containers:
        - name: nginx
          image: nginx:latest
        - name: exporter
          image: nginx-exporter:latest
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: web-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web-app
  updatePolicy:
    updateMode: Recreate
  resourcePolicy:
    containerPolicies:
      - containerName: nginx
        minAllowed:
          cpu: 10m
          memory: 50Mi
        maxAllowed:
          cpu: 1000m
          memory: 1Gi
      - containerName: exporter
        minAllowed:
          cpu: 10m
          memory: 20Mi
        maxAllowed:
          cpu: 100m
          memory: 100Mi
```

✅ **Correct** - VPA with updateMode Off (no container policies required):

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app
  namespace: d8-my-module
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  updatePolicy:
    updateMode: "Off"  # ✅ No resourcePolicy required for monitoring-only mode
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      vpa:
        - kind: Deployment
          name: standby-holder  # Exclude specific deployment
        - kind: StatefulSet
          name: temporary-workload
```

---

### pdb

**Purpose:** Ensures Deployments and StatefulSets have PodDisruptionBudgets (PDB) to maintain availability during voluntary disruptions like node drains, upgrades, or cluster maintenance. This prevents service outages during routine operations.

**Description:**

Validates that every Deployment and StatefulSet (excluding DaemonSets) has a corresponding PodDisruptionBudget that matches the pod's labels. The PDB ensures a minimum number of pods remain available during disruptions.

**What it checks:**

1. Every Deployment and StatefulSet has a PDB
2. PDB selector matches pod template labels
3. PDB and controller are in the same namespace
4. PDB has no Helm hook annotations (hooks run before regular resources)

**Why it matters:**

Without PDBs:
- Cluster operations can take down all replicas simultaneously
- Applications become unavailable during maintenance
- Rolling updates may violate availability requirements
- Kubernetes cannot safely evict pods

**Examples:**

❌ **Incorrect** - Deployment without PDB:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
  namespace: d8-my-module
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-server
  template:
    metadata:
      labels:
        app: api-server
    spec:
      containers:
        - name: api
          image: api-server:v1.0.0
```

**Error:**
```
Error: No PodDisruptionBudget found for controller
```

❌ **Incorrect** - PDB with mismatched labels:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
  namespace: d8-my-module
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-server
  template:
    metadata:
      labels:
        app: api-server
        version: v1
    spec:
      containers:
        - name: api
          image: api-server:v1.0.0
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api-server-pdb
  namespace: d8-my-module
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: api-server
      version: v2  # ❌ Version mismatch
```

**Error:**
```
Error: No PodDisruptionBudget matches pod labels of the controller
Value: app=api-server,version=v1
```

❌ **Incorrect** - PDB with Helm hooks:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api-server-pdb
  namespace: d8-my-module
  annotations:
    helm.sh/hook: pre-install  # ❌ Hooks are not allowed
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: api-server
```

**Error:**
```
Error: PDB must have no helm hook annotations
```

✅ **Correct** - Deployment with PDB:

```yaml
# templates/deployment.yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
  namespace: d8-my-module
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-server
  template:
    metadata:
      labels:
        app: api-server
    spec:
      containers:
        - name: api
          image: api-server:v1.0.0
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api-server-pdb
  namespace: d8-my-module
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: api-server
```

✅ **Correct** - StatefulSet with maxUnavailable PDB:

```yaml
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: database
  namespace: d8-my-module
spec:
  replicas: 3
  serviceName: database
  selector:
    matchLabels:
      app: database
      component: storage
  template:
    metadata:
      labels:
        app: database
        component: storage
    spec:
      containers:
        - name: postgres
          image: postgres:14
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: database-pdb
  namespace: d8-my-module
spec:
  maxUnavailable: 1  # ✅ Alternative to minAvailable
  selector:
    matchLabels:
      app: database
      component: storage
```

✅ **Correct** - Multiple labels matching:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: d8-my-module
spec:
  replicas: 5
  selector:
    matchLabels:
      app: frontend
      tier: web
      environment: production
  template:
    metadata:
      labels:
        app: frontend
        tier: web
        environment: production
    spec:
      containers:
        - name: nginx
          image: nginx:latest
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: frontend-pdb
  namespace: d8-my-module
spec:
  minAvailable: 3
  selector:
    matchLabels:
      app: frontend
      tier: web
      environment: production
```

**Note:** DaemonSets are automatically excluded from PDB validation since they run one pod per node and have different availability semantics.

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      pdb:
        - kind: Deployment
          name: single-replica-app  # Exclude single-replica workloads
        - kind: StatefulSet
          name: test-database
```

---

### kube-rbac-proxy

**Purpose:** Ensures all Deckhouse system namespaces contain the kube-rbac-proxy CA certificate ConfigMap. This certificate is required for secure mTLS communication between components using kube-rbac-proxy for authentication and authorization.

**Description:**

Validates that every namespace starting with `d8-` contains a ConfigMap named `kube-rbac-proxy-ca.crt`. This ConfigMap provides the CA certificate needed for secure communication with kube-rbac-proxy sidecars.

**What it checks:**

1. All namespaces with `d8-` prefix
2. Presence of ConfigMap `kube-rbac-proxy-ca.crt` in each namespace
3. Recommends using `helm_lib_kube_rbac_proxy_ca_certificate` helper

**Why it matters:**

kube-rbac-proxy provides authentication and authorization for Kubernetes components. Without the CA certificate:
- mTLS connections fail
- Metrics endpoints become unavailable
- Security is compromised

**Examples:**

❌ **Incorrect** - Namespace without CA certificate:

```yaml
# templates/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: d8-my-module
  labels:
    heritage: deckhouse
    module: my-module
```

**Error:**
```
Error: All system namespaces should contain kube-rbac-proxy CA certificate.
       Consider using corresponding helm_lib helper 'helm_lib_kube_rbac_proxy_ca_certificate'.
Object: namespace = d8-my-module
```

✅ **Correct** - Using Helm library helper:

```yaml
# templates/namespace.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-my-module
  labels:
    heritage: deckhouse
    module: my-module
---
{{- include "helm_lib_kube_rbac_proxy_ca_certificate" . }}
```

The `helm_lib_kube_rbac_proxy_ca_certificate` helper automatically creates the required ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy-ca.crt
  namespace: d8-my-module
  labels:
    heritage: deckhouse
    module: my-module
data:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

✅ **Correct** - Manual ConfigMap creation:

```yaml
# templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy-ca.crt
  namespace: d8-my-module
data:
  ca.crt: {{ .Values.global.discovery.kubeRBACProxyCA | quote }}
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      kube-rbac-proxy:
        - d8-system  # Exclude specific namespace
        - d8-test-namespace
```

---

### service-port

**Purpose:** Ensures Services use named target ports instead of numeric ports. Named ports make configurations more maintainable, self-documenting, and resistant to changes in container port numbers.

**Description:**

Validates that all Service port definitions use named `targetPort` values (strings) rather than numeric values (integers). This improves readability and allows changing container ports without updating Service definitions.

**What it checks:**

1. Every Service port has a `targetPort` field
2. `targetPort` is a named port (string), not a number
3. Named ports should match container port names in pods

**Why it matters:**

Named ports:
- Make Service definitions self-documenting ("https" vs "8443")
- Allow changing container ports without updating Services
- Improve configuration clarity and maintainability
- Reduce errors when multiple ports exist

**Examples:**

❌ **Incorrect** - Numeric target port:

```yaml
# templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: web-service
  namespace: d8-my-module
spec:
  selector:
    app: web
  ports:
    - name: http
      port: 80
      targetPort: 8080  # ❌ Numeric port
      protocol: TCP
```

**Error:**
```
Error: Service port must use a named (non-numeric) target port
Object: Service/web-service ; port = http
Value: 8080
```

❌ **Incorrect** - Zero/empty target port:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-service
  namespace: d8-my-module
spec:
  selector:
    app: api
  ports:
    - name: https
      port: 443
      targetPort: 0  # ❌ Invalid port
      protocol: TCP
```

**Error:**
```
Error: Service port must use an explicit named (non-numeric) target port
Object: Service/api-service ; port = https
```

✅ **Correct** - Named target port:

```yaml
# templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: web-service
  namespace: d8-my-module
spec:
  selector:
    app: web
  ports:
    - name: http
      port: 80
      targetPort: http  # ✅ Named port
      protocol: TCP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: d8-my-module
spec:
  template:
    spec:
      containers:
        - name: nginx
          ports:
            - name: http  # ✅ Matches Service targetPort
              containerPort: 8080
              protocol: TCP
```

✅ **Correct** - Multiple named ports:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: application
  namespace: d8-my-module
spec:
  selector:
    app: application
  ports:
    - name: http
      port: 80
      targetPort: http  # ✅ Named
      protocol: TCP
    - name: https
      port: 443
      targetPort: https  # ✅ Named
      protocol: TCP
    - name: metrics
      port: 9090
      targetPort: metrics  # ✅ Named
      protocol: TCP
```

✅ **Correct** - Service with pod definition:

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: database
  namespace: d8-my-module
spec:
  selector:
    app: postgres
  ports:
    - name: postgresql
      port: 5432
      targetPort: postgres  # ✅ Named
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: d8-my-module
spec:
  template:
    spec:
      containers:
        - name: postgres
          image: postgres:14
          ports:
            - name: postgres  # ✅ Container port name matches Service
              containerPort: 5432
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      service-port:
        - d8-control-plane-apiserver  # Exclude specific service
        - legacy-service
```

---

### ingress-rules

**Purpose:** Ensures Ingress resources include required security configuration snippets, specifically the Strict-Transport-Security (HSTS) header for enforcing HTTPS connections.

**Description:**

Validates that Ingress objects with `nginx.ingress.kubernetes.io/configuration-snippet` annotation contain the required HSTS header configuration using the `helm_lib_module_ingress_configuration_snippet` helper.

**What it checks:**

1. Ingresses with `nginx.ingress.kubernetes.io/configuration-snippet` annotation
2. Configuration snippet contains `add_header Strict-Transport-Security`
3. Recommends using `helm_lib_module_ingress_configuration_snippet` helper

**Why it matters:**

HSTS (HTTP Strict-Transport-Security):
- Forces browsers to use HTTPS only
- Prevents protocol downgrade attacks
- Protects against man-in-the-middle attacks
- Required for security compliance

**Examples:**

❌ **Incorrect** - Missing HSTS header:

```yaml
# templates/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: d8-my-module
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_set_header X-Custom-Header "value";
      # ❌ Missing Strict-Transport-Security header
spec:
  rules:
    - host: dashboard.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: dashboard
                port:
                  name: http
```

**Error:**
```
Error: Ingress annotation "nginx.ingress.kubernetes.io/configuration-snippet" does not contain required snippet "{{ include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }}".
Object: dashboard
```

✅ **Correct** - Using Helm library helper:

```yaml
# templates/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dashboard
  namespace: d8-my-module
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
{{- include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }}
      # Additional custom configuration if needed
      proxy_set_header X-Custom-Header "value";
spec:
  rules:
    - host: dashboard.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: dashboard
                port:
                  name: http
```

The helper includes the HSTS header:

```nginx
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
```

✅ **Correct** - Ingress without configuration-snippet (not checked):

```yaml
# templates/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: simple-ingress
  namespace: d8-my-module
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    # ✅ No configuration-snippet, so rule doesn't apply
spec:
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app
                port:
                  name: http
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      ingress-rules:
        - kind: Ingress
          name: dashboard  # Exclude specific Ingress
        - kind: Ingress
          name: legacy-api
```

---

### prometheus-rules

**Purpose:** Validates Prometheus alerting and recording rules using promtool and ensures proper Helm template structure. This catches syntax errors, invalid queries, and ensures monitoring rules are properly packaged.

**Description:**

Validates PrometheusRule objects and the monitoring template structure. Uses promtool to check rule validity and ensures the module's `monitoring.yaml` template uses the correct Helm library helper for rendering Prometheus rules.

**What it checks:**

1. PrometheusRule objects pass promtool validation
2. PromQL expressions are syntactically correct
3. Rule groups are properly structured
4. Module with `monitoring/prometheus-rules` folder has `templates/monitoring.yaml`
5. `templates/monitoring.yaml` uses `helm_lib_prometheus_rules` helper

**Why it matters:**

Invalid Prometheus rules:
- Fail to load into Prometheus
- Cause alerts to never fire
- Waste resources on broken queries
- Create blind spots in monitoring

**Examples:**

❌ **Incorrect** - Invalid PromQL:

```yaml
# monitoring/prometheus-rules/alerts.yaml
- name: my-module.alerts
  rules:
    - alert: HighErrorRate
      expr: rate(errors_total[5m) > 0.05  # ❌ Missing closing bracket
      for: 5m
      labels:
        severity: warning
      annotations:
        description: Error rate is above 5%
```

**Error:**
```
Error: Promtool check failed for Prometheus rule: unclosed left bracket
```

❌ **Incorrect** - Missing monitoring.yaml template:

```
monitoring/
└── prometheus-rules/
    └── alerts.yaml
templates/
└── deployment.yaml
# ❌ Missing templates/monitoring.yaml
```

**Error:**
```
Error: Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file
```

❌ **Incorrect** - Wrong template content:

```yaml
# templates/monitoring.yaml
apiVersion: v1
kind: ConfigMap  # ❌ Wrong approach
metadata:
  name: prometheus-rules
data:
  alerts.yaml: |
    # ...
```

**Error:**
```
Error: The content of the 'templates/monitoring.yaml' should be equal to:
{{- include "helm_lib_prometheus_rules" (list . "YOUR NAMESPACE TO DEPLOY RULES: d8-monitoring, d8-system or module namespace") }}
```

✅ **Correct** - Valid PrometheusRule with proper template:

```yaml
# monitoring/prometheus-rules/alerts.yaml
- name: my-module.alerts
  rules:
    - alert: HighErrorRate
      expr: rate(errors_total[5m]) > 0.05
      for: 5m
      labels:
        severity: warning
        module: my-module
      annotations:
        summary: High error rate detected
        description: Error rate is {{ $value | humanizePercentage }} (threshold: 5%)

    - alert: ServiceDown
      expr: up{job="my-module"} == 0
      for: 5m
      labels:
        severity: critical
        module: my-module
      annotations:
        summary: Service is down
        description: "Service {{ $labels.instance }} has been down for more than 5 minutes"
```

```yaml
# templates/monitoring.yaml
{{- include "helm_lib_prometheus_rules" (list . "d8-monitoring") }}
```

✅ **Correct** - Recording rules:

```yaml
# monitoring/prometheus-rules/recording-rules.yaml
- name: my-module.recording
  interval: 30s
  rules:
    - record: my_module:request_duration_seconds:p99
      expr: histogram_quantile(0.99, rate(request_duration_seconds_bucket[5m]))
      labels:
        module: my-module

    - record: my_module:error_rate:5m
      expr: rate(errors_total[5m]) / rate(requests_total[5m])
      labels:
        module: my-module
```

✅ **Correct** - Multiple rule files:

```
monitoring/
└── prometheus-rules/
    ├── alerts.yaml
    ├── recording-rules.yaml
    └── slo-rules.yaml
```

```yaml
# templates/monitoring.yaml
{{- include "helm_lib_prometheus_rules" (list . "d8-system") }}
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    prometheus-rules:
      disable: true  # Disable rule completely
```

---

### grafana-dashboards

**Purpose:** Validates Grafana dashboard templates are properly structured and ensures the module's `monitoring.yaml` uses the correct Helm library helper for dashboard definitions.

**Description:**

Validates that modules with Grafana dashboards in the `monitoring/grafana-dashboards` folder have a properly configured `templates/monitoring.yaml` that uses the `helm_lib_grafana_dashboard_definitions` helper.

**What it checks:**

1. Module with `monitoring/grafana-dashboards` folder has `templates/monitoring.yaml`
2. `templates/monitoring.yaml` uses `helm_lib_grafana_dashboard_definitions` helper
3. Template content matches expected format

**Why it matters:**

Proper dashboard packaging ensures:
- Dashboards are correctly deployed as ConfigMaps
- Grafana can discover and load dashboards
- Dashboard updates are properly propagated
- Consistent dashboard management across modules

**Examples:**

❌ **Incorrect** - Missing monitoring.yaml:

```
monitoring/
└── grafana-dashboards/
    └── main.json
templates/
└── deployment.yaml
# ❌ Missing templates/monitoring.yaml
```

**Error:**
```
Error: Module with the 'monitoring' folder should have the 'templates/monitoring.yaml' file
```

❌ **Incorrect** - Wrong template content:

```yaml
# templates/monitoring.yaml
apiVersion: v1
kind: ConfigMap  # ❌ Manual ConfigMap creation
metadata:
  name: grafana-dashboards
data:
  main.json: |
    {{ .Files.Get "monitoring/grafana-dashboards/main.json" }}
```

**Error:**
```
Error: The content of the 'templates/monitoring.yaml' should be equal to:
{{- include "helm_lib_grafana_dashboard_definitions" . }}
```

✅ **Correct** - Proper dashboard template:

```yaml
# templates/monitoring.yaml
{{- include "helm_lib_grafana_dashboard_definitions" . }}
```

This helper automatically:
- Creates ConfigMap with dashboard JSON files
- Adds proper labels for Grafana discovery
- Handles multiple dashboards
- Sets correct namespace

✅ **Correct** - Multiple dashboards:

```
monitoring/
└── grafana-dashboards/
    ├── overview.json
    ├── detailed-metrics.json
    └── troubleshooting.json
```

```yaml
# templates/monitoring.yaml
{{- include "helm_lib_grafana_dashboard_definitions" . }}
```

All dashboards are automatically included.

✅ **Correct** - Recursive dashboard structure:

```
monitoring/
└── grafana-dashboards/
    ├── main/
    │   └── overview.json
    └── detailed/
        └── metrics.json
```

```yaml
# templates/monitoring.yaml
{{- include "helm_lib_grafana_dashboard_definitions_recursion" (list . "monitoring/grafana-dashboards") }}
```

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  templates:
    grafana:
      disable: true  # Disable rule completely
```

---

### cluster-domain

**Purpose:** Prevents hardcoding of the cluster domain (`cluster.local`) in templates. Ensures cluster domain is configurable to support custom cluster configurations and multi-cluster deployments.

**Description:**

Scans all template files (`.yaml`, `.yml`, `.tpl`) for hardcoded `cluster.local` strings and recommends using the dynamic `.Values.global.clusterConfiguration.clusterDomain` value instead.

**What it checks:**

1. All files in `templates/` directory
2. Presence of hardcoded `cluster.local` string
3. Recommends using `.Values.global.clusterConfiguration.clusterDomain`

**Why it matters:**

Hardcoded cluster domains:
- Break in custom domain configurations
- Prevent multi-cluster deployments
- Make templates non-portable
- Cause DNS resolution failures

**Examples:**

❌ **Incorrect** - Hardcoded cluster.local:

```yaml
# templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: d8-my-module
data:
  database_url: "postgres://db.d8-my-module.svc.cluster.local:5432/mydb"
  # ❌ Hardcoded cluster.local
```

**Error:**
```
Error: File contains hardcoded 'cluster.local' substring. Use '.Values.global.clusterConfiguration.clusterDomain' instead for dynamic cluster domain configuration.
Object: templates/configmap.yaml
```

❌ **Incorrect** - Hardcoded in service URL:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  template:
    spec:
      containers:
        - name: worker
          env:
            - name: API_ENDPOINT
              value: "http://api.d8-my-module.svc.cluster.local:8080"
              # ❌ Hardcoded cluster domain
```

**Error:**
```
Error: File contains hardcoded 'cluster.local' substring. Use '.Values.global.clusterConfiguration.clusterDomain' instead for dynamic cluster domain configuration.
Object: templates/deployment.yaml
```

✅ **Correct** - Dynamic cluster domain in ConfigMap:

```yaml
# templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: d8-my-module
data:
  database_url: "postgres://db.d8-my-module.svc.{{ .Values.global.clusterConfiguration.clusterDomain }}:5432/mydb"
```

✅ **Correct** - Dynamic cluster domain in Deployment:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  template:
    spec:
      containers:
        - name: worker
          env:
            - name: API_ENDPOINT
              value: "http://api.d8-my-module.svc.{{ .Values.global.clusterConfiguration.clusterDomain }}:8080"
            - name: CLUSTER_DOMAIN
              value: {{ .Values.global.clusterConfiguration.clusterDomain | quote }}
```

✅ **Correct** - Using Helm helper for full URLs:

```yaml
# templates/_helpers.tpl
{{- define "my-module.serviceFQDN" -}}
{{- printf "%s.%s.svc.%s" .serviceName .namespace .Values.global.clusterConfiguration.clusterDomain -}}
{{- end -}}
```

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: DATABASE_HOST
              value: {{ include "my-module.serviceFQDN" (dict "serviceName" "postgres" "namespace" "d8-my-module" "Values" .Values) }}
```

---

### registry

**Purpose:** Validates registry secret configuration for modules using custom container registries. Ensures modules properly configure both global and module-specific registry authentication.

**Description:**

Checks the `registry-secret.yaml` file for proper configuration of Docker registry credentials. When using global registry configuration, validates that module-specific registry configuration is also present.

**What it checks:**

1. Presence of `registry-secret.yaml` file (if it exists)
2. If file uses `.Values.global.modulesImages.registry.dockercfg`
3. Ensures corresponding module-specific `.Values.<moduleName>.registry.dockercfg` is also configured
4. Module name is converted to camelCase for values reference

**Why it matters:**

Proper registry configuration:
- Enables pulling from private registries
- Allows module-specific registry overrides
- Ensures authentication credentials are available
- Supports air-gapped deployments

**Examples:**

❌ **Incorrect** - Only global registry configured:

```yaml
# templates/registry-secret.yaml
{{- if .Values.global.modulesImages.registry.dockercfg }}
apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
  namespace: d8-my-module
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ .Values.global.modulesImages.registry.dockercfg }}
{{- end }}
```

**Error:**
```
Error: registry-secret.yaml file contains .Values.global.modulesImages.registry.dockercfg but missing .Values.myModule.registry.dockercfg
```

✅ **Correct** - Both global and module-specific registry:

```yaml
# templates/registry-secret.yaml
{{- if or .Values.global.modulesImages.registry.dockercfg .Values.myModule.registry.dockercfg }}
apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
  namespace: d8-my-module
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ .Values.myModule.registry.dockercfg | default .Values.global.modulesImages.registry.dockercfg }}
{{- end }}
```

✅ **Correct** - Module-specific registry only:

```yaml
# templates/registry-secret.yaml
{{- if .Values.myModule.registry.dockercfg }}
apiVersion: v1
kind: Secret
metadata:
  name: registry-secret
  namespace: d8-my-module
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ .Values.myModule.registry.dockercfg }}
{{- end }}
```

✅ **Correct** - No registry-secret.yaml (uses default):

```
templates/
├── deployment.yaml
├── service.yaml
└── ...
# ✅ No registry-secret.yaml - uses cluster default
```

**Module Name Conversion:**

Module names are converted to camelCase for values:
- `my-module` → `myModule`
- `cert-manager` → `certManager`
- `ingress-nginx` → `ingressNginx`

---

### werf

**Purpose:** Validates werf.yaml templates to ensure proper git stage dependencies are configured. This ensures consistent and reproducible image builds.

**Description:**

Scans the werf.yaml file for image definitions that belong to the module and validates that git sections have proper `stageDependencies` configured.

**What it checks:**

1. Image definitions in werf.yaml that match the module name
2. Git sections have `stageDependencies` configured

**Why it matters:**

Missing stage dependencies:
- Can cause inconsistent builds
- May skip rebuilds when source files change
- Lead to stale images being used
- Create debugging difficulties

**Examples:**

❌ **Incorrect** - Missing stageDependencies:

```yaml
# werf.yaml
image: my-module/app
git:
  - add: /src
    to: /app
    # ❌ Missing stageDependencies
```

**Error:**
```
Error: parsing Werf file, document 1 (image: my-module/app) failed: 'git.stageDependencies' is required
```

✅ **Correct** - With stageDependencies:

```yaml
# werf.yaml
image: my-module/app
git:
  - add: /src
    to: /app
    stageDependencies:
      install:
        - "**/*"
      beforeSetup:
        - "go.mod"
        - "go.sum"
```

✅ **Correct** - Multiple git sources:

```yaml
# werf.yaml
image: my-module/app
git:
  - add: /src
    to: /app/src
    stageDependencies:
      install:
        - "**/*.go"
  - add: /config
    to: /app/config
    stageDependencies:
      beforeSetup:
        - "**/*.yaml"
```

## Configuration

The Templates linter can be configured at the module level with rule-specific settings and exclusions.

### Module-Level Settings

Configure the overall impact level and individual rule toggles:

```yaml
# .dmt.yaml
linters-settings:
  templates:
    # Overall impact level
    impact: error  # Options: error, warning, info, ignored
    
    # Disable specific rules
    grafana-dashboards:
      disable: true
    
    prometheus-rules:
      disable: true
```

### Rule-Level Exclusions

Configure exclusions for specific rules:

```yaml
# .dmt.yaml
linters-settings:
  templates:
    exclude-rules:
      # VPA exclusions (by kind and name)
      vpa:
        - kind: Deployment
          name: standby-holder-name
        - kind: StatefulSet
          name: temporary-workload
      
      # PDB exclusions (by kind and name)
      pdb:
        - kind: Deployment
          name: single-replica-app
        - kind: StatefulSet
          name: test-database
      
      # Ingress exclusions (by kind and name)
      ingress-rules:
        - kind: Ingress
          name: dashboard
        - kind: Ingress
          name: legacy-api
      
      # Service port exclusions (by service name)
      service-port:
        - d8-control-plane-apiserver
        - legacy-service
      
      # kube-rbac-proxy exclusions (by namespace)
      kube-rbac-proxy:
        - d8-system
        - d8-test-namespace
```

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  templates:
    # Global impact level
    impact: error
    
    # Disable monitoring rules during development
    grafana-dashboards:
      disable: false
    
    prometheus-rules:
      disable: false
    
    # Rule-specific exclusions
    exclude-rules:
      vpa:
        - kind: Deployment
          name: test-deployment
        - kind: DaemonSet
          name: log-collector
      
      pdb:
        - kind: Deployment
          name: single-pod-app
      
      ingress-rules:
        - kind: Ingress
          name: internal-dashboard
      
      service-port:
        - apiserver
        - webhook-service
      
      kube-rbac-proxy:
        - d8-development
```

### Configuration in Module Directory

Place `.dmt.yaml` in your module directory for module-specific settings:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  templates:
    impact: warning  # More lenient for this module
    
    grafana-dashboards:
      disable: true  # No dashboards yet
    
    exclude-rules:
      vpa:
        - kind: Deployment
          name: experimental-feature
```

## Common Issues

### Issue: VPA not found for controller

**Symptom:**
```
Error: No VPA is found for object
```

**Cause:** Deployment, DaemonSet, or StatefulSet missing corresponding VerticalPodAutoscaler.

**Solutions:**

1. **Create VPA for the controller:**

   ```yaml
   apiVersion: autoscaling.k8s.io/v1
   kind: VerticalPodAutoscaler
   metadata:
     name: my-app
     namespace: d8-my-module
   spec:
     targetRef:
       apiVersion: apps/v1
       kind: Deployment
       name: my-app
     updatePolicy:
       updateMode: Recreate
     resourcePolicy:
       containerPolicies:
         - containerName: "*"
           minAllowed:
             cpu: 10m
             memory: 50Mi
           maxAllowed:
             cpu: 1000m
             memory: 1Gi
   ```

2. **Exclude the controller from VPA validation:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     templates:
       exclude-rules:
         vpa:
           - kind: Deployment
             name: my-app
   ```

### Issue: Container missing in VPA resourcePolicy

**Symptom:**
```
Error: The container should have corresponding VPA resourcePolicy entry
Object: Deployment/my-app ; container = sidecar
```

**Cause:** VPA resourcePolicy doesn't include all containers from the pod template.

**Solutions:**

1. **Add missing container to VPA:**

   ```yaml
   resourcePolicy:
     containerPolicies:
       - containerName: app
         minAllowed:
           cpu: 10m
           memory: 50Mi
         maxAllowed:
           cpu: 500m
           memory: 500Mi
       - containerName: sidecar  # Add missing container
         minAllowed:
           cpu: 5m
           memory: 20Mi
         maxAllowed:
           cpu: 100m
           memory: 100Mi
   ```

2. **Use wildcard for all containers:**

   ```yaml
   resourcePolicy:
     containerPolicies:
       - containerName: "*"  # Matches all containers
         minAllowed:
           cpu: 10m
           memory: 50Mi
         maxAllowed:
           cpu: 1000m
           memory: 1Gi
   ```

### Issue: PDB not found for controller

**Symptom:**
```
Error: No PodDisruptionBudget found for controller
```

**Cause:** Deployment or StatefulSet missing PodDisruptionBudget.

**Solutions:**

1. **Create PDB:**

   ```yaml
   apiVersion: policy/v1
   kind: PodDisruptionBudget
   metadata:
     name: my-app-pdb
     namespace: d8-my-module
   spec:
     minAvailable: 1
     selector:
       matchLabels:
         app: my-app
   ```

2. **Exclude from PDB validation:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     templates:
       exclude-rules:
         pdb:
           - kind: Deployment
             name: my-app
   ```

### Issue: PDB selector doesn't match pod labels

**Symptom:**
```
Error: No PodDisruptionBudget matches pod labels of the controller
Value: app=my-app,version=v1
```

**Cause:** PDB selector doesn't match all pod template labels.

**Solutions:**

1. **Fix PDB selector to match pod labels:**

   ```yaml
   # Deployment labels
   template:
     metadata:
       labels:
         app: my-app
         version: v1
   
   # PDB selector should match
   spec:
     selector:
       matchLabels:
         app: my-app
         version: v1
   ```

2. **Use subset of labels in PDB:**

   ```yaml
   # PDB can match subset of pod labels
   spec:
     selector:
       matchLabels:
         app: my-app  # Just match app label
   ```

### Issue: Service using numeric target port

**Symptom:**
```
Error: Service port must use a named (non-numeric) target port
Object: Service/web-service ; port = http
Value: 8080
```

**Cause:** Service targetPort is numeric instead of named.

**Solutions:**

1. **Change to named port:**

   ```yaml
   # Service
   ports:
     - name: http
       port: 80
       targetPort: http  # Use name
   
   # Pod
   containers:
     - name: app
       ports:
         - name: http  # Define name
           containerPort: 8080
   ```

### Issue: Missing kube-rbac-proxy CA certificate

**Symptom:**
```
Error: All system namespaces should contain kube-rbac-proxy CA certificate.
Object: namespace = d8-my-module
```

**Cause:** Namespace missing required ConfigMap for kube-rbac-proxy.

**Solutions:**

1. **Use Helm library helper:**

   ```yaml
   {{- include "helm_lib_kube_rbac_proxy_ca_certificate" . }}
   ```

2. **Exclude namespace from validation:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     templates:
       exclude-rules:
         kube-rbac-proxy:
           - d8-my-module
   ```

### Issue: Ingress missing HSTS configuration

**Symptom:**
```
Error: Ingress annotation "nginx.ingress.kubernetes.io/configuration-snippet" does not contain required snippet
```

**Cause:** Ingress configuration-snippet missing Strict-Transport-Security header.

**Solutions:**

1. **Use Helm helper:**

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/configuration-snippet: |
{{- include "helm_lib_module_ingress_configuration_snippet" . | nindent 6 }}
   ```

2. **Exclude Ingress:**

   ```yaml
   # .dmt.yaml
   linters-settings:
     templates:
       exclude-rules:
         ingress-rules:
           - kind: Ingress
             name: dashboard
   ```

### Issue: Invalid Prometheus rules

**Symptom:**
```
Error: Promtool check failed for Prometheus rule: unclosed left bracket
```

**Cause:** PromQL syntax error in Prometheus rules.

**Solutions:**

1. **Fix PromQL expression:**

   ```yaml
   # Before (incorrect)
   expr: rate(errors_total[5m) > 0.05
   
   # After (correct)
   expr: rate(errors_total[5m]) > 0.05
   ```

2. **Test rules locally:**

   ```bash
   promtool check rules monitoring/prometheus-rules/*.yaml
   ```

### Issue: Hardcoded cluster domain

**Symptom:**
```
Error: File contains hardcoded 'cluster.local' substring. Use '.Values.global.clusterConfiguration.clusterDomain' instead
Object: templates/configmap.yaml
```

## Grafana Dashboard Validation Rules

The linter now includes comprehensive validation for Grafana dashboards based on best practices from the Deckhouse project:

### Deprecated Panel Types

- **graph** → **timeseries**: The `graph` panel type is deprecated and should be replaced with `timeseries`
- **flant-statusmap-panel** → **state-timeline**: The custom statusmap panel should use the standard `state-timeline` panel

### Deprecated Intervals

- **interval_rv**, **interval_sx3**, **interval_sx4**: These custom intervals are deprecated and should be replaced with Grafana's built-in `$__rate_interval` variable

### Legacy Alert Rules

- **Built-in alerts**: Panels with embedded alert rules should use external Alertmanager instead of Grafana's built-in alerting

### Datasource Validation

- **Legacy format**: Detects old datasource UID formats that need to be resaved with newer Grafana versions
- **Hardcoded UIDs**: Identifies hardcoded datasource UIDs that should use Grafana variables
- **Prometheus UIDs**: Ensures Prometheus datasources use recommended UID patterns (`$ds_prometheus` or `${ds_prometheus}`)

### Template Variables

- **Required variable**: Ensures dashboards contain the required `ds_prometheus` variable of type `datasource`
- **Query variables**: Validates that query variables use recommended datasource UIDs
