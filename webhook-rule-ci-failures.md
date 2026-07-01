# CI Failure Analysis: `webhook-configuration-annotations` rule

PR: [#412](https://github.com/deckhouse/dmt/pull/412) | Run: [#134](https://github.com/deckhouse/dmt/actions/runs/28246712494)

## Summary

The new `webhook-configuration-annotations` rule defaults to **`error`** level. When run against the Deckhouse `main` codebase, it flags **19 webhook resource files across 9 modules** that lack the required `werf.io/weight` or `werf.io/deploy-dependency` annotation. This causes the `DMT Lint Verify` CI job to fail.

## Affected Modules

| # | Module | Files | Kind | CODEOWNERS |
|---|--------|-------|------|------------|
| 1 | **002-deckhouse** | `templates/admission/validation.yaml` | ValidatingWebhookConfiguration | @ldmonster |
| 2 | **015-admission-policy-engine** | `templates/validatingwebhookconfiguration.yaml` | ValidatingWebhookConfiguration | @nabokihms @AlwxSin |
| 3 | **015-admission-policy-engine** | `templates/mutatingwebhookconfiguration.yaml` | MutatingWebhookConfiguration | @nabokihms @AlwxSin |
| 4 | **030-cloud-provider-vcd** | `templates/capcd-controller-manager/admission.yaml` | ValidatingWebhookConfiguration, MutatingWebhookConfiguration | @aleksey-su @pabateman |
| 5 | **040-node-manager** | `templates/node-controller/webhook.yaml` | ValidatingWebhookConfiguration | @borg-z @090809 |
| 6 | **040-node-manager** | `templates/caps-controller-manager/webhook.yaml` | ValidatingWebhookConfiguration | @borg-z @090809 |
| 7 | **042-kube-dns** | `templates/sts-pods-hosts-appender-webhook/webhook-configuration.yaml` | MutatingWebhookConfiguration | @AndreyPavlovFlant @apolovov |
| 8 | **101-cert-manager** | `templates/webhook/validatingwebhookconfiguration.yaml` | ValidatingWebhookConfiguration | @Suselz @AlwxSin |
| 9 | **101-cert-manager** | `templates/webhook/mutatingwebhookconfiguration.yaml` | MutatingWebhookConfiguration | @Suselz @AlwxSin |
| 10 | **110-istio** | `templates/control-plane/mutatingwebhook-global.yaml` | MutatingWebhookConfiguration | @skurbatov @apolovov |
| 11 | **110-istio** | `templates/control-plane/validatingwebhook-global.yaml` | ValidatingWebhookConfiguration | @skurbatov @apolovov |
| 12 | **110-istio** | `templates/control-plane/mutatingwebhook-revisions.yaml` | MutatingWebhookConfiguration | @skurbatov @apolovov |
| 13 | **110-istio** | `templates/control-plane/validatingwebhook-revisions.yaml` | ValidatingWebhookConfiguration | @skurbatov @apolovov |
| 14 | **110-istio** | `templates/control-plane/iop/iop.yaml` | ValidatingWebhookConfiguration, MutatingWebhookConfiguration | @skurbatov @apolovov |
| 15 | **110-istio** | `templates/control-plane/iop/istios.yaml` | ValidatingWebhookConfiguration, MutatingWebhookConfiguration | @skurbatov @apolovov |
| 16 | **160-multitenancy-manager** | `templates/admission/validation.yaml` | ValidatingWebhookConfiguration | @nabokihms @AlwxSin |
| 17 | **160-multitenancy-manager** | `templates/cluster-objects-controller/protect-webhook.yaml` | ValidatingWebhookConfiguration | @nabokihms @AlwxSin |
| 18 | **302-vertical-pod-autoscaler** | `templates/admission-controller/mutatingwebhookconfiguration.yaml` | MutatingWebhookConfiguration | @yalosev @ergoz |

## Already Compliant

Only one file already has the required annotation:

| Module | File | Annotation |
|--------|------|------------|
| **040-node-manager** | `templates/capi-controller-manager/webhook.yaml` | `werf.io/deploy-dependency` (×4) |

## Root Cause

In `pkg/config.go`, the `RuleConfig.SetLevel()` method defaults to `Error` when no configuration is provided:

```go
func (rc *RuleConfig) SetLevel(level, backoff string) {
    if level != "" { ...; return }
    if backoff != "" { ...; return }
    lvl := Error  // ← hardcoded default
    rc.impact = &lvl
}
```

Neither the global `.dmtlint.yaml` nor any module-level config specify an impact for `webhook-configuration-annotations`, so the rule always runs at `error` level.

## Fix Options

1. **Lower default to `warn`** in `pkg/config.go` — change `lvl := Error` → `lvl := Warn`
2. **Add to Deckhouse global `.dmtlint.yaml`**:
   ```yaml
   global:
     linters-settings:
       templates:
         rules:
           webhook-configuration-annotations:
             impact: warn
   ```
