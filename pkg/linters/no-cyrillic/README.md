## Description

Checks that there are no cyrillic characters in the source code.


## Settings example

### Module level

```yaml
linters-settings:
  no-cyrillic:
    files:
      exclude:
        - /path/to/file.go
        - /path/to/file.yaml
    dirs:
      exclude:
        - /path/to/dir
```