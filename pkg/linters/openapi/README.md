## Description

Checks that all openapi file is valid.

## Settings example

### Module level

```yaml
linters-settings:
  openapi:
    exclude-rules:
      enum:
        - "properties.storageClass.properties.provision.items.properties.type"
        - "properties.storageClass.properties.provision.items.oneOf[*].properties.type"
      ha-absolute-keys:
        - "properties.storageClass.properties.provision.items.properties.type"
      key-banned-names:
        - "properties.storageClass.properties.provision.items.properties.type"
    impact: error
```
