# Conversions Tester

Validates that a module's OpenAPI configuration conversions are well-formed and that they transform settings as expected.

## Overview

The **Conversions Tester** runs against every module that ships an `openapi/conversions/` directory. It checks the conversion files for validity, makes sure the declared config version stays in sync with the conversions, and replays any testcases through the converter to confirm they produce the expected output.

It is invoked through the [`dmt test conversions`](../../../internal/test/README.md) command.

A module is *applicable* (i.e. tested) only when it has an `openapi/conversions/` directory; other modules are skipped.

## Checks

- ✅ The latest conversion version matches `x-config-version` in `openapi/config-values.yaml`
- ✅ `x-config-version` is set whenever conversions exist
- ✅ All conversion files in `openapi/conversions/` are valid
- ✅ Each testcase in `openapi/conversions/testcases.yaml` converts its input settings into the expected output

## File Structure

```
openapi/
├── config-values.yaml          # must declare x-config-version
└── conversions/
    ├── v2.yaml                  # conversion to version 2 (first conversion)
    ├── v3.yaml                  # conversion to version 3
    └── testcases.yaml           # optional: conversion testcases
```

## Conversion File

```yaml
# openapi/conversions/v2.yaml
version: 2
conversions:
  - del(.auth.password)
description:
  en: "v2"
  ru: "v2"
```

## Testcases File

Each testcase declares the source and target versions, the input `settings`, and the `expected` result after conversion.

```yaml
# openapi/conversions/testcases.yaml
testcases:
  - name: "should delete auth.password on 1 to 2"
    currentVersion: 1
    expectedVersion: 2
    settings: |
      auth:
        password: secret
    expected: |
      auth: {}
```

A module without `testcases.yaml` still has its conversion files and version validated.

## Usage

```bash
# Test conversions for all modules under the current directory
dmt test conversions

# Test a single module
dmt test conversions ./modules/my-module
```
