## Description

Checks the copyright header in files.
Check oss info in the `oss.yaml` file.

## Settings example

### Module level

```yaml
linters-settings:
  license:
    oss:
      disable: true
    copyright:
      files:
        exclude:
          - /images/upmeter/stress.sh
          - /images/simple-bridge/rootfs/bin/simple-bridge
      dirs:
        exclude:
          - /images
```