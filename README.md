# d8-lint

## config

Example settings:

```yaml
linters-settings:
  probes:
    probes-excludes:
      d8-istio:
        - kube-rbac-proxy
        - operator
  openapi:
    enum-file-excludes:
      - prometheus:/openapi/values.yaml:
          - "properties.internal.properties.grafana.properties.alertsChannelsConfig.properties.notifiers.items.properties.type"
  nocyrillic:
    no-cyrillic-file-excludes:
      - user-authz:/rbac.yaml
      - documentation:/images/web/site/_data/topnav.yml
  license:
    copyright-excludes:
      - upmeter:/images/upmeter/stress.sh
      - cni-simple-bridge:/images/simple-bridge/rootfs/bin/simple-bridge
    skip-oss-checks:
      - 001-priority-class
  rbac:
    skip-check-wildcards:
      - "admission-policy-engine/templates/rbac-for-us.yaml":
          - "d8:admission-policy-engine:gatekeeper"
  helm:
    skip-module-image-name:
      - "021-cni-cilium/images/cilium/Dockerfile"
      - "021-cni-cilium/images/virt-cilium/Dockerfile"
    skip-distroless-image-check:
      - "base-cilium-dev/werf.inc.yaml"
      - "cilium-envoy/werf.inc.yaml"
  container:
    skip-containers:
      - "okmeter:okagent"
      - "d8-control-plane-manager:*.image-holder"
warnings-only:
  - openapi
  - no-cyrillic
  - copyright
  - probes

```
