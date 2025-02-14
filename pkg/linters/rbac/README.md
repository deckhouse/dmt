## Description

Checks RBACv1 (deprecated) rules

## Settings example

## Module level

```yaml
linters-settings:
  rbac:
    exclude-rules:
      # exclude if object kind and object name equals
      wildcards:
        - kind: ClusterRole
          name: d8:deckhouse:webhook-handler
      # exclude if object kind and object name equals
      placement:
        - kind: ClusterRole
          name: d8:rbac-proxy
  impact: error
```
