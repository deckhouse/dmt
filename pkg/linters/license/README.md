## Description

Checks the copyright header in files.

## Settings example

### Module level

```yaml
linters-settings:
  license:
    copyright:
      files:
        exclude:
          - /images/upmeter/stress.sh
          - /images/simple-bridge/rootfs/bin/simple-bridge
      dirs:
        exclude:
          - /images
```