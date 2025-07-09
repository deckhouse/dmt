# Exclusions Package

This package provides a universal exclusion tracking mechanism for DMT linters.

## Usage

### Universal Constructor

For any rule type, use the universal constructor with the corresponding key generator:

```go
import (
    "github.com/deckhouse/dmt/pkg/exclusions"
    "github.com/deckhouse/dmt/pkg"
)

tracker := exclusions.NewExclusionTracker()
excludeRules := []pkg.KindRuleExclude{
    {Kind: "Deployment", Name: "test-deployment"},
}

rule := exclusions.NewTrackedRule(
    pkg.NewKindRuleWithTracker(excludeRules, tracker, "linterID", "ruleID"),
    exclusions.KindRuleKeys(excludeRules),
    tracker, "linterID", "ruleID", "moduleName",
)

if rule.Enabled("Deployment", "test-deployment") {
    // ...
}
```

### Key Generators

- `exclusions.StringRuleKeys([]pkg.StringRuleExclude)`
- `exclusions.PrefixRuleKeys([]pkg.PrefixRuleExclude)`
- `exclusions.KindRuleKeys([]pkg.KindRuleExclude)`
- `exclusions.ContainerRuleKeys([]pkg.ContainerRuleExclude)`
- `exclusions.ServicePortRuleKeys([]pkg.ServicePortExclude)`
- `exclusions.PathRuleKeys([]pkg.StringRuleExclude, []pkg.PrefixRuleExclude)`

### Example for PathRule

```go
stringRules := []pkg.StringRuleExclude{"skip1", "skip2"}
prefixRules := []pkg.PrefixRuleExclude{"prefix-"}

rule := exclusions.NewTrackedRule(
    pkg.NewPathRuleWithTracker(stringRules, prefixRules, tracker, "linterID", "ruleID"),
    exclusions.PathRuleKeys(stringRules, prefixRules),
    tracker, "linterID", "ruleID", "moduleName",
)
```

### For BoolRule

```go
rule := exclusions.NewTrackedRule(
    pkg.NewBoolRuleWithTracker(true, tracker, "linterID", "ruleID"),
    nil, // or []string{}
    tracker, "linterID", "ruleID", "moduleName",
)
```

## Benefits

- Single point for tracking and registering exclusions
- No code duplication
- Easy to extend and maintain
- Aliases for old functions are kept for compatibility, but it's recommended to use only the universal approach

## Tests

```bash
go test ./pkg/exclusions -v
```

## Migration

1. Replace calls to NewTracked*Rule* with the universal constructor and corresponding key generator.
2. For new rules, use only the universal approach. 