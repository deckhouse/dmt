## Description

- Check PodDisruptionBudgets described
- Check VerticalPodAutoscaler described
- Check kube-rbac-proxy CA certificate exists
- Check container settings # перенести сюда containers linter
  - containers with the duplicated names
  - containers with the duplicated env variables
  - misconfigured images repository and digest
  - imagePullPolicy is "Always" (should be unspecified or "IfNotPresent")
  - ephemeral storage is not defined in .resources
  - SecurityContext is not defined
  - container uses port <= 1024



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