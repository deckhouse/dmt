# Templates Tester

Renders a module's templates with per-case values and compares the result against committed golden snapshots, in the spirit of Deckhouse's Helm testing harness.

## Overview

The **Templates Tester** runs against every module that ships a `templates-tests/` directory. For each test case it renders the module's chart with the case's values, normalizes the output, and compares it byte-for-byte against the committed `expected.yaml` snapshot.

It is invoked through the [`dmt test templates`](../../../internal/test/README.md) command.

A module is *applicable* (i.e. tested) only when it has a `templates-tests/` directory with at least one case; other modules are skipped.

## Flags

| Flag | Description |
|------|-------------|
| `--update` | Regenerate (overwrite) the golden snapshots instead of comparing against them. |

## File Structure

Each direct subdirectory of `templates-tests/` is a test case. The values file is optional; the snapshot is the expected rendered output.

```
my-module/
├── templates/
│   └── configmap.yaml
└── templates-tests/
    └── basic/
        ├── values.yaml          # optional: values for this case
        └── expected.yaml        # golden snapshot
```

## How Comparison Works

The module is rendered with the case's `values.yaml`, then the output is normalized into a deterministic, canonical YAML stream: files sorted by path, each document re-marshalled with sorted keys and prefixed with its source path. The normalized output is compared byte-for-byte against `expected.yaml`.

## Example

Given this template:

```yaml
# templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.app.name }}
  namespace: {{ .Release.Namespace }}
data:
  greeting: {{ .Values.app.greeting | quote }}
  replicas: {{ .Values.app.replicas | quote }}
```

and these case values:

```yaml
# templates-tests/basic/values.yaml
app:
  name: demo
  greeting: hello
  replicas: 3
```

the committed snapshot is:

```yaml
# templates-tests/basic/expected.yaml
---
# Source: e2e-templates/templates/configmap.yaml
apiVersion: v1
data:
  greeting: hello
  replicas: "3"
kind: ConfigMap
metadata:
  name: demo
  namespace: d8-e2e-templates
```

## Usage

```bash
# Compare all modules' templates against their snapshots
dmt test templates

# Test a single module
dmt test templates ./modules/my-module

# Create or refresh snapshots after intentional template changes
dmt test templates ./modules/my-module --update
```

> When a snapshot is missing, the case fails and reports the rendered output. Run with `--update` to create it, then review and commit the result.
