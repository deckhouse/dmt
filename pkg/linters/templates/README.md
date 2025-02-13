## Description

- Check VerticalPodAutoscaler described
templates(+): переносим pdb + vpa, переносим objectServiceTargetPort из Containers, переносим kube-rbac-proxy, переносим monitoring

## Settings example

## Module level

```yaml
linters-settings:
  vpa:
    exclude-controllers:
      - node-manager
      - okmeter
```
