## Description

- Check VerticalPodAutoscaler described
- Check PodDisruptionBudgets described
- Check kube-rbac-proxy CA certificate exists
templates(+): переносим pdb + vpa, переносим objectServiceTargetPort из Containers, переносим kube-rbac-proxy, переносим monitoring

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
