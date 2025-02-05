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
