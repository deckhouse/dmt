## Description

Checks that all openapi file is valid.

## Settings example

### Module level

This linter has settings.

```yaml
linters-settings:
  openapi:
    exclude-rules:
      enum:
        - "properties.storageClass.properties.provision.items.properties.type"
        - "properties.storageClass.properties.provision.items.oneOf[*].properties.type"
      ha-absolute-keys:
        - "properties.storageClass.properties.provision.items.properties.type"
      key-banned-names:
        - "properties.storageClass.properties.provision.items.properties.type"
      crd-names:
        - projects.deckhouse.io
        - projecttemplates.deckhouse.io
    impact: error
```

## Enum

From [kubernetes/api-conventions][1]:

> Some fields will have a list of allowed values (enumerations). These values will be strings, and they will be in
> CamelCase, with an initial uppercase letter. Examples: ClusterFirst, Pending, ClientIP.
>
> When an acronym or initialism each letter in the acronym should be uppercase, such as with ClientIP or TCPDelay.
> 
> When a proper name or the name of a command-line executable is used as a constant the proper name should be
> represented in consistent casing - examples: systemd, iptables, IPVS, cgroupfs, Docker (as a generic concept), docker
> (as the command-line executable). If a proper name is used which has mixed capitalization like eBPF that should be
> preserved in a longer constant such as eBPFDelegation.

[1]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#constants