# dmt end-to-end test framework

This package runs **dmt** against concrete module fixtures and asserts on the
structured findings it produces. Unlike the per-rule unit tests (which call a
single rule in isolation), each case here exercises config loading, the helm
render, and every linter together — the same path a user hits when running
`dmt lint <module>`.

Most cases run the full `dmt lint` pipeline. Cases can also run the
`dmt test conversions` testers by setting `kind: conversions` in
`expected.yaml`; their results are surfaced through the same matching machinery
under the linter ID `conversions`.

## How it works

Cases live under `testdata/<linter>/<case>/`, where `<linter>` is the name of
the linter the case primarily exercises. `TestE2E` (in `e2e_test.go`) treats
each linter folder as a parent subtest and every case inside it as a nested
subtest (e.g. `TestE2E/templates/vpa-pdb-absent`). A directory is a case when it
contains an `expected.yaml` file. For each case it:

1. copies the case's `module/` directory into an isolated temp dir (so the run
   is hermetic — no `.dmtlint.yaml` inherited from parent dirs, no artifacts
   written back into `testdata/`),
2. runs the lint `manager` (or, for `kind: conversions`, the conversions
   testers) against the copy,
3. matches the collected findings against the expectations in `expected.yaml`.

## Adding a case

Create a new directory under the relevant linter folder,
`testdata/<linter>/<your-case>/`:

```
testdata/<linter>/<your-case>/
├── expected.yaml          # the case specification
└── module/                # the Deckhouse module that gets linted
    ├── module.yaml
    ├── openapi/
    │   ├── values.yaml
    │   └── config-values.yaml
    └── templates/         # optional
        └── ...
```

A minimal lint-able module needs at least:

- `module.yaml` with `name` and `namespace` (or `Chart.yaml` + a `.namespace`
  file), and
- an `openapi/` directory containing `values.yaml` and `config-values.yaml`.

### `expected.yaml`

```yaml
description: >
  Human-readable summary of what this case verifies.
kind: lint              # "lint" (default) or "conversions"
module: module          # subdir to lint, defaults to "module"
expectClean: false      # assert the run produced zero findings
exhaustive: false       # assert there are NO findings beyond those listed
expect:
  - linter: container             # required, matched case-insensitively
    rule: env-variables-duplicates # optional
    level: error                   # optional: ignored | warn | error
    textContains: "same name"      # optional, case-sensitive substring
    count: 1                       # optional; 0/omitted means "at least one"
```

Matching semantics:

- `linter` is required and compared case-insensitively to the finding's linter ID.
- `rule`, `level` and `textContains` are optional filters; when present they all
  must match.
- `count` is the expected number of matching findings. `0` (or omitting it)
  means "at least one".
- `expectClean: true` asserts the module produced no findings at all.
- `exhaustive: true` asserts that every produced finding is matched by some
  entry in `expect` (use it to lock down the complete output of a fixture).

### Discovering the findings for a new fixture

The quickest way to author expectations is to temporarily set
`expectClean: true` in `expected.yaml` and run the case — the failure message
prints every finding the fixture produced, which you can then copy into
`expect`:

```bash
go test ./test/e2e/ -run 'TestE2E/<linter>/<your-case>' -v
```

## Current cases

| Case | Exercises |
|------|-----------|
| `container/bad-deployment` | container linter (labels, security context, probes, resources, image digest, duplicate env, seccomp) |
| `module/missing-metadata` | module linter (definition-file, helmignore) + documentation linter (readme) |
| `module/disable-message-deprecated` | module linter `definition-file` (deprecated `disable.message`, use localized `disable.messages`) |
| `no-cyrillic/in-template` | no-cyrillic linter (Cyrillic characters in a yaml file) |
| `no-cyrillic/skip-russian-files` | no-cyrillic linter skips Russian localized files (`*.ru.yml`, `*.ru.yaml`, `*.ru.json`, `doc-ru-*.yml`) while still reporting a regular Cyrillic template |
| `no-cyrillic/skip-filenames-extensions` | no-cyrillic linter skips every filename/path pattern (`doc-ru-*`, `*.ru.{yaml,yml,json,md,html}`, `*_RU.md`, `docs/site/_*`, `docs/documentation/_*`, `tools/spelling/*`, `openapi/conversions/*`, `module.yaml`, `i18n/*`, `ru.*`) and non-scanned extensions (`.txt`), reporting only one genuine Cyrillic template |
| `rbac/wildcards` | rbac linter (wildcards in a Role) |
| `hooks/ingress` | hooks linter (Ingress without copy_custom_certificate hook) |
| `openapi/bilingual` | openapi linter (missing doc-ru- translation, missing CRD module label) |
| `images/werf` | images linter (werf fromImage not under base/) |
| `documentation/missing-readme` | documentation linter `readme` (no docs/README.md) |
| `documentation/cyrillic-in-english` | documentation linter `cyrillic-in-english` (Cyrillic in English docs) |
| `documentation/missing-russian` | documentation linter `bilingual` (English doc without a .ru.md counterpart) |

### conversions tester (`kind: conversions`)

| Case | Exercises |
|------|-----------|
| `conversions/passing` | conversion testcases that match the converter output (no errors) |
| `conversions/with-conversions` | conversions with multiple passing testcases incl. a no-op (no errors) |
| `conversions/failing` | a testcase whose expected result does not match the conversion output |
| `conversions/version-mismatch` | `x-config-version` does not match the latest conversion version |

### templates linter (comprehensive)

| Case | Rule(s) exercised |
|------|-------------------|
| `templates/service-port` | `service-port` (numeric Service target port) |
| `templates/vpa-pdb-absent` | `vpa` (no VPA), `pdb` (no PDB) for a controller |
| `templates/vpa-misconfigured` | `vpa` (updateMode `Auto`, missing `resourcePolicy.containerPolicies`) |
| `templates/pdb-mismatch` | `pdb` (PDB selector does not match controller pod labels) |
| `templates/pdb-helm-hook` | `pdb` (PDB carries helm hook annotations) |
| `templates/ingress-snippet` | `ingress-rules` (configuration-snippet missing HSTS) |
| `templates/monitoring-missing-yaml` | `prometheus-rules` + `grafana-dashboards` (monitoring/ without templates/monitoring.yaml) |
| `templates/grafana-dashboard` | `grafana-dashboards` (deprecated panel type, missing prometheus datasource variable) |
| `templates/prometheus-promtool` | `prometheus-rules` (invalid PromQL via promtool) |
| `templates/kube-rbac-proxy` | `kube-rbac-proxy` (d8-* namespace without kube-rbac-proxy CA) |
| `templates/cluster-domain` | `cluster-domain` (hardcoded `cluster.local`) |
| `templates/registry` | `registry` (global dockercfg without module override) |
| `templates/enabled-modules` | `enabled-modules` (deprecated `.Values.global.enabledModules | has`) |

## Running

```bash
# all cases
go test ./test/e2e/

# every case for one linter
go test ./test/e2e/ -run 'TestE2E/templates' -v

# a single case, verbose
go test ./test/e2e/ -run 'TestE2E/templates/vpa-pdb-absent' -v
```
