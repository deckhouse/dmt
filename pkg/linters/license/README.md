## Description

Checks the copyright header in files.

## Settings example

### Module level

This linter has settings.

```yaml
linters-settings:
  license:
    exclude-rules:
      files:
        - /images/upmeter/stress.sh
        - /images/simple-bridge/rootfs/bin/simple-bridge
        - directory:/hooks/venv/
```