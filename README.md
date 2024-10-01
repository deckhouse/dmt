# d8-lint

## config

Example settings:

```yaml
linters-settings:
  probes:
    probes-excludes:
      d8-istio:
        - "kube-rbac-proxy"
        - "operator"
  openapi:
    enum-file-excludes:
      - prometheus:/openapi/values.yaml:
        - "properties.internal.properties.grafana.properties.alertsChannelsConfig.properties.notifiers.items.properties.type"
      - cloud-provider-aws:/openapi/values.yaml:
        - "properties.internal.properties.storageClass.properties.provision.items.properties.type"
        - "properties.internal.properties.storageClasses.items.oneOf[*].properties.type"
      - cloud-provider-aws:/openapi/config-values.yaml:
        - "properties.storageClass.properties.provision.items.properties.type"
        - "properties.storageClass.properties.provision.items.oneOf[*].properties.type"
  copyright:
    copyright-excludes:
      - "upmeter:/images/upmeter/stress.sh"
      - "cni-simple-bridge:/images/simple-bridge/rootfs/bin/simple-bridge"
  nocyrillic:
    no-cyrillic-file-excludes:
      - "user-authz:/rbac.yaml"
      - "documentation:/images/web/site/_data/topnav.yml"

warnings-only:
  - openapi
  - no-cyrillic
  - copyright
  - probes
```
