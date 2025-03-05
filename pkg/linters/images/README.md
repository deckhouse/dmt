## Description

- Check werf.yaml file
- Check Dockerfile
- Check that images are distroless
- Check patch rules for images

## Settings example

## Module level

This linter has the following settings:

```yaml
linters-settings:
  images:
    patches:
      disable: false
    impact: error
```
