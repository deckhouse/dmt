## Description

Checks for probes defined in containers.

## Settings example

### Module level

```yaml
linters-settings:
  probes:
    liveness:
      exclude-containers:
        - kube-rbac-proxy
    readiness:
      exclude-containers:
        - kube-rbac-proxy
```