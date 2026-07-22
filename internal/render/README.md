# Render Command

Render Deckhouse module templates to disk, exactly as `dmt` renders them internally while linting.

## Overview

`dmt render` discovers every module under a given path (including subdirectories) and renders each module's `templates/` directory using values generated from the module's OpenAPI schemas. It is useful for inspecting the manifests a module actually produces, for diffing changes between revisions, and for feeding rendered output into other tooling.

A directory is treated as a renderable module only when it contains a `templates/` directory. Directories that ship a `module.yaml` but no `templates/` (for example, CRD definitions that merely share the name) are skipped.

## Usage

```bash
dmt render [module-path] [flags]
```

- `module-path` (optional): directory to scan for modules. Defaults to the current directory (`.`).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--output` | `-o` | Directory to write the rendered output into. Created if absent; a `rendered` subdirectory is created inside it. When omitted, each module is rendered into a `rendered/` directory at its own root. |

## Values

Values are generated from each module's OpenAPI schemas, the same way `dmt` generates values while linting:

- `openapi/config-values.yaml`
- `openapi/values.yaml`

Global values are taken from the surrounding Deckhouse repository root when one is found above the module (a directory that ships `global-hooks/openapi/config-values.yaml` and `global-hooks/openapi/values.yaml` alongside a `modules/` directory). Otherwise, the embedded defaults are used.

## Editions

When a module ships edition-specific values schemas following the `openapi/values_<edition>.yaml` convention, the output is split per edition. The base `openapi/values.yaml` is always rendered as the `default` edition.

For example, a module with `openapi/values.yaml`, `openapi/values_ce.yaml`, and `openapi/values_ee.yaml` produces `default`, `ce`, and `ee` editions.

## Output Layout

### Per-module mode (default)

Without `--output`, each module is rendered into a `rendered/` directory at the module root. The directory is recreated on every run.

Without edition-specific schemas:

```
my-module/
└── rendered/
    └── templates/
        └── ...           # rendered manifests
```

With edition-specific schemas:

```
my-module/
└── rendered/
    ├── default/          # from openapi/values.yaml
    │   └── templates/...
    ├── ce/               # from openapi/values_ce.yaml
    │   └── templates/...
    └── ee/               # from openapi/values_ee.yaml
        └── templates/...
```

### Shared-output mode (`--output`)

With `--output <dir>`, all modules are rendered into a shared tree under `<dir>/rendered/<module-name>/<edition>/`. The module name is taken from the module's `module.yaml` (falling back to `Chart.yaml`, then the directory name). Each module's subtree is recreated on every run, and the `default` edition is always present.

```
<output>/
└── rendered/
    └── my-module/
        ├── default/
        │   └── templates/...
        └── ce/
            └── templates/...
```

## Examples

```bash
# Render every module under the current directory, in-place
dmt render

# Render every module under ./modules
dmt render ./modules

# Render a single module
dmt render ./modules/my-module

# Render all modules into a shared output directory
dmt render ./modules --output ./build

# Increase verbosity to see which modules are rendered or skipped
dmt render ./modules --log-level debug
```

## Notes

- Empty rendered files (whitespace only) are omitted from the output.
- If no modules are found under the path, `dmt render` prints a warning and exits successfully.
- If one or more modules fail to render, the command logs each failure and exits with a non-zero status after attempting all modules.
