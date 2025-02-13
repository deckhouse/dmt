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
      read-only-root-filesystem:
        - kind: Deployment
          name: deckhouse
          container: init-downloaded-modules
      resources:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      security-context:
        - kind: Deployment
          name: caps-controller-manager
          container: caps-controller-manager
        - kind: Deployment
          name: standby-holder-name
      dns-policy:
        - kind: Deployment
          name: machine-controller-manager
      service-port:
        - d8-control-plane-apiserver
      liveness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
      readiness-probe:
        - kind: Deployment
          name: standby-holder-name
          container: reserve-resources
```