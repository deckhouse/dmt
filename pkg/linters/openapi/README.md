## Description

Checks openapi spec:
 - Enum values have to be started with a Capital letter
 - Enum values have to be unique
 - some keys should not have a default value


## Settings example

### Module level

```yaml
linters-settings:
  openapi:
    enum:
      exclude-file-keys:
        - "properties.internal.properties.providerDiscoveryData.properties.apiVersion"
        - "properties.storageClass.properties.compatibilityFlag"
    ha-and-https:
      exclude-keys:
        - properties.publishAPI.properties.https
    key-name:
      prohibited-names:
        - default
```