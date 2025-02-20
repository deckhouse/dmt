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

## Linters list

| Linter                                                   | Description                                                                  |
|----------------------------------------------------------|------------------------------------------------------------------------------|
| [container](pkg/linters/container/README.md)             | Check containers - duplicated names, env variables, ports, security context, liveness and readiness probes.|
| [hooks](pkg/linters/hooks/README.md)                     | Check hooks rules. |
| [images](pkg/linters/images/README.md)                   | Check images build instructions. |
| [module](pkg/linters/module/README.md)                   | Check module.yaml definition, openapi conversions, oss.yaml file.|
| [no-cyrillic](pkg/linters/no-cyrillic/README.md)         | Check cyrillic letters. |
| [openapi](pkg/linters/openapi/README.md)                 | Check openapi settings, crds. |
| [rbac](pkg/linters/rbac/README.md)                       | Check rbac rules. |
| [templates](pkg/linters/templates/README.md)             | Check templates rules, VPA, PDB settings, prometheus, grafana rules, kube-rbac-proxy, service target port. |

## Configuration

You can exclude linters or setup them via the config file `.dmtlint.yaml`

### Global settings:

```yaml
global:  
  linters:
    probes:
      impact: warn | critical
    images:
      impact: warn | critical  
```
