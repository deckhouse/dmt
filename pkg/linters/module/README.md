## Description

- Checks the `module.yaml` definition file.
- Check that openapi conversions have human-readable description
- Check oss info in the `oss.yaml` file.
- Check license header in files.

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
