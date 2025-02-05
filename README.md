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
| [container](pkg/linters/container/README.md)             | Check containers - Duplicated names, env variables, ports, security context. |
| [conversions](pkg/linters/conversions/README.md)         | Check openapi conversions.                                                   |
| [images](pkg/linters/conversions/README.md)              | Check images build instructions.                                             |
| [ingress](pkg/linters/ingress/README.md)                 | Check ingress TLS hook settings.                                             |
| [pdb](pkg/linters/pdb/README.md)                         | Check PDB settings.                                                          |
| [vpa](pkg/linters/vpa/README.md)                         | Check VPA settings.                                                          |
| [kube-rbac-proxy](pkg/linters/kube-rbac-proxy/README.md) | TODO.                                                                        |
| [license](pkg/linters/license/README.md)                 | Check license header in files.                                               |
| [oss](pkg/linters/oss/README.md)                         | Check oss.yaml file exists.                                                  |
| [module](pkg/linters/module/README.md)                   | Check module.yaml definition.                                                |
| [monitoring](pkg/linters/monitoring/README.md)           | Check prometheus rules.                                                      |
| [no-cyrillic](pkg/linters/no-cyrillic/README.md)         | Check cyrillic letters.                                                      |
| [openapi](pkg/linters/openapi/README.md)                 | Check openapi settings.                                                      |
| [probes](pkg/linters/probes/README.md)                   | Check liveness and readiness probes.                                         |
| [rbac](pkg/linters/rbac/README.md)                       | Check rbac rules.                                                            |

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
