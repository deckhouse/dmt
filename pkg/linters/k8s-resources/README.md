## Description

- Check PodDisruptionBudgets described
- Check VerticalPodAutoscaler described
- Check kube-rbac-proxy CA certificate exists

## Settings example

## Module level

```yaml
linters-settings:
  k8s-resources:
    pdb:
      exclude-controllers:
        - node-manager
        - okmeter
    vpa:
      exclude-controllers:
        - node-manager
        - okmeter
    containers:
      exclude:
        - "cloud-controller-manager"
        - "dashboard-metrics-scraper"
```