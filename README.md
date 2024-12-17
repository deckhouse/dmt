# dmt

Deckhouse Module Tool - the swiss knife for your Deckhouse modules

### How to use

#### Lint

You can run linter checks for a module:
```shell
dmt lint /some/path/<your-module>
```
or some pack of modules
```shell
dmt lint /some/path/
```
where `/some/path/` looks like this:
```shell
ls -l /some/path/
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 10 21:46 001-module-one
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 12 21:45 002-module-two
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 10 21:46 003-module-three
```


#### Gen

Generate some automatic rules for you module
<Coming soon>



## Configuration

You can exclude linters or setup them via the config file `.dmtlint.yaml`. This config file can be either in the module directory or in any of the top-level directories up to /.

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
      - other-module:/external/**/*.txt
  license:
    copyright-excludes:
      - upmeter:/images/upmeter/stress.sh
      - upmeter:/hooks/.venv/**/*
      - cni-simple-bridge:/images/simple-bridge/rootfs/bin/simple-bridge
    skip-oss-checks:
      - 001-priority-class
  rbac:
    skip-check-wildcards:
      - "admission-policy-engine/templates/rbac-for-us.yaml":
          - "d8:admission-policy-engine:gatekeeper"
    skip-module-check-binding:
      - "user-authz"
    skip-object-check-binding:
      - "user-authz"
      - "deckhouse"
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
  monitoring:
    skip-module-checks:
      - "340-extended-monitoring"
      - "030-cloud-provider-yandex"
warnings-only:
  - openapi
  - no-cyrillic
  - copyright
  - probes
```
