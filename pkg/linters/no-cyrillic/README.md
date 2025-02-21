## Description

Checks that there are no cyrillic characters in the source code.

## Settings example

### Module level

This linter has settings.

```yaml
linters-settings:
  no-cyrillic:
    exclude-rules:
      files:
        - path/to/file.go
        - path/to/file.yaml
      directories:
        - path/to/dir
```