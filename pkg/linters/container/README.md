## Description

Checks containers inside the template spec. This linter protects against the next cases:
 - containers with the duplicated names
 - containers with the duplicated env variables
 - misconfigured images repository and digest
 - imagePullPolicy is "Always" (should be unspecified or "IfNotPresent")
 - ephemeral storage is not defined in .resources
 - SecurityContext is not defined
 - container uses port <= 1024
- Checks for probes defined in containers.

# Сюда перенести из linters/k8s-resources


## Settings example

### Module level

```yaml
linters-settings:
  container:
    exclude-rules:
      # exclude if object kind, object name and containers name are equal
      read-only-root-filesystem:
        - kind: Deployment
          name: deckhouse
          container: init-downloaded-modules
      # exclude if object kind, object name and containers name are equal
      resources:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      # exclude if object kind, object name and containers name are equal
      security-context:
        - kind: Deployment
          name: caps-controller-manager
          container: caps-controller-manager
      # exclude if object kind, object name equals. affect any containers within
        - kind: Deployment
          name: standby-holder-name
      # exclude if object kind, object name are equal
      dns-policy:
        - kind: Deployment
          name: machine-controller-manager
      # exclude if object kind, object name and containers name are equal
      liveness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      # exclude if object kind, object name and containers name are equal
      readiness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
    impact: error
```