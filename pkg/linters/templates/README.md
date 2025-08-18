## Description

- Check VerticalPodAutoscaler described
- Check PodDisruptionBudgets described
- Check kube-rbac-proxy CA certificate exists
Lint monitoring rules:
- run promtool checks
- render prometheus rules
- validate grafana dashboards with comprehensive rules

## Settings example

## Module level

This linter has settings.

```yaml
linters-settings:
  templates:
    # disable grafana-dashboards rule
    grafana-dashboards:
      disable: true
    exclude-rules:
      # exclude if target ref equals one of
      vpa:
        - kind: Deployment
          name: standby-holder-name
      # exclude if target ref equals one of
      pdb:
        - kind: Deployment
          name: standby-holder-name
          # exclude if target ref equals one of
      ingress-rules:
        - kind: Ingress
          name: dashboard
      # exclude if service name equals one of
      service-port:
        - d8-control-plane-apiserver
      # exclude if object namespace equals one of
      kube-rbac-proxy:
        - d8-system
    impact: error
```

## Grafana Dashboard Validation Rules

The linter now includes comprehensive validation for Grafana dashboards based on best practices from the Deckhouse project:

### Deprecated Panel Types

- **graph** → **timeseries**: The `graph` panel type is deprecated and should be replaced with `timeseries`
- **flant-statusmap-panel** → **state-timeline**: The custom statusmap panel should use the standard `state-timeline` panel

### Deprecated Intervals

- **interval_rv**, **interval_sx3**, **interval_sx4**: These custom intervals are deprecated and should be replaced with Grafana's built-in `$__rate_interval` variable

### Legacy Alert Rules

- **Built-in alerts**: Panels with embedded alert rules should use external Alertmanager instead of Grafana's built-in alerting

### Datasource Validation

- **Legacy format**: Detects old datasource UID formats that need to be resaved with newer Grafana versions
- **Hardcoded UIDs**: Identifies hardcoded datasource UIDs that should use Grafana variables
- **Prometheus UIDs**: Ensures Prometheus datasources use recommended UID patterns (`$ds_prometheus` or `${ds_prometheus}`)

### Template Variables

- **Required variable**: Ensures dashboards contain the required `ds_prometheus` variable of type `datasource`
- **Query variables**: Validates that query variables use recommended datasource UIDs
