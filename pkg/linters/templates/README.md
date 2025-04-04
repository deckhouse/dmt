## Description

- Check VerticalPodAutoscaler described
- Check PodDisruptionBudgets described
- Check kube-rbac-proxy CA certificate exists
Lint monitoring rules:
- run promtool checks
- render prometheus rules

## Settings example

## Module level

This linter has settings.

```yaml
linters-settings:
  templates:
    exclude-rules:
      # exclude if target ref equals one of
      vpa:
        - kind: Deployment
          name: standby-holder-name
      # exclude if target ref equals one of
      pdb:
        - kind: Deployment
          name: standby-holder-name
          # exclude if target ref equals one of
      ingress-rules:
        - kind: Ingress
          name: dashboard
      # exclude if service name equals one of
      service-port:
        - d8-control-plane-apiserver
      # exclude if object namespace equals one of
      kube-rbac-proxy:
        - d8-system
    impact: error
```
