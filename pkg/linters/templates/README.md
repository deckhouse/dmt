## Description

- Check VerticalPodAutoscaler described
- Check PodDisruptionBudgets described
- Check kube-rbac-proxy CA certificate exists
Lint monitoring rules:
- run promtool checks
- render prometheus rules

## Settings example

## Module level

```yaml
linters-settings:
  templates:
    exclude-rules:
      vpa:
        - kind: Deployment
          name: standby-holder-name
      pdb:
        - kind: Deployment
          name: standby-holder-name
      service-port:
        - d8-control-plane-apiserver
      kube-rbac-proxy:
        - d8-system
```
