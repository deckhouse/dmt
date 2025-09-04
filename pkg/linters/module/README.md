## Description

- Checks the `module.yaml` definition file.
- Check that openapi conversions have human-readable description
- Check oss info in the `oss.yaml` file.
- Check license header in files.
- Validates `accessibility` section in `module.yaml` files.
- Validates `update` section in `module.yaml` files.

### Accessibility Validation

The linter validates the optional `accessibility` section in `module.yaml` files:

#### Valid Editions

- `ce` - Community Edition
- `fe` - Free Edition  
- `ee` - Enterprise Edition
- `se` - Standard Edition
- `se-plus` - Standard Edition Plus
- `be` - Business Edition
- `_default` - Default behavior override

#### Valid Bundles

- `Minimal` - Minimal bundle
- `Managed` - Managed bundle
- `Default` - Default bundle

#### Validation Rules

- `accessibility.editions` is required when `accessibility` is specified
- Each edition must have valid `available` (boolean) and `enabledInBundles` (array) fields
- `enabledInBundles` must contain only valid bundle names
- Edition names must be from the valid editions list

#### Example

```yaml
accessibility:
  editions:
    _default:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
    ee:
      available: true
      enabledInBundles:
        - Minimal
        - Managed
        - Default
```

### Update Validation

The linter validates the optional `update` section in `module.yaml` files:

#### Validation Rules

- Each version entry must have both `from` and `to` fields populated
- The `to` version must be greater than the `from` version 
- Versions must be in `major.minor` format (patch versions are not allowed)
- Version entries must be sorted by `from` version ascending, then by `to` version ascending
- For the same `to` version, there must not be duplicate `from` versions

#### Example

```yaml
update:
  versions:
    - from: "1.16"
      to: "1.20"
    - from: "1.16"
      to: "1.25"
    - from: "1.17"
      to: "1.20"
```

## Settings example

### Module level

This linter has the following settings:

```yaml
linters-settings:
  module:
    oss:
      disable: false
    deinition-file:
      disable: false
    conversions:
      disable: false
    exclude-rules:
      license:
        files:
          - images/upmeter/stress.sh
          - images/simple-bridge/rootfs/bin/simple-bridge
        directories:
          - hooks/venv/
    impact: error
```
