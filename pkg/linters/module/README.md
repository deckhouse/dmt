## Description

- Checks module.yaml definition file.
- Check that openapi conversions have human-readable description
- Check oss info in the `oss.yaml` file.

## Settings example

### Module level

This linter has settings.

```yaml
linters-settings:
  module:
    oss:
      disable: false
    module-yaml:
      disable: false
    conversions:
      disable: false
    impact: error
```
