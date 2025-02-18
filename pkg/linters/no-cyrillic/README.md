## Description

Checks that there are no cyrillic characters in the source code.

## Settings example

### Module level

This linter has settings.

```yaml
linters-settings:
  no-cyrillic:
    files:
      exclude:
        - path/to/file.go
        - path/to/file.yaml
    directories:
      exclude:
        - path/to/dir
```