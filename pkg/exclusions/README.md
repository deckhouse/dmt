# Exclusion Tracking System

The exclusion tracking system allows determining which configured exclusions are not used during linter operation and outputs them in the final log as warnings.

## Description

In the DMT project, a configuration is implemented that presents exclusions for linters. When running, these exclusions are used to determine which errors to include in the result log and which ones to skip.

The new functionality allows:
- Tracking the usage of each exclusion during linter operation
- Determining which exclusions were not used
- Associating exclusions with specific modules for better tracking
- Outputting a list of unused exclusions in the final log as warnings with module information

## Architecture

### ExclusionTracker

The main component of the system is `ExclusionTracker`, which:
- Registers all configured exclusions for each linter and rule
- Tracks exclusion usage during execution
- Associates exclusions with specific modules
- Provides methods for getting usage statistics and unused exclusions

### Tracked Rules

Extended versions of rules have been created that inherit from base rules and add tracking functionality:
- `TrackedStringRule` - for string exclusions
- `TrackedPrefixRule` - for prefix-based exclusions
- `TrackedKindRule` - for object type and name exclusions
- `TrackedContainerRule` - for container exclusions
- `TrackedServicePortRule` - for service port exclusions
- `TrackedPathRule` - for path-based exclusions
- `TrackedBoolRule` - for boolean exclusions

Each tracked rule type has both regular and module-specific constructors:
- `NewTracked*Rule()` - for general use
- `NewTracked*RuleForModule()` - for module-specific tracking

## Usage

### In Linter Manager

The linter manager initializes the exclusion tracker and passes it to linters:

```go
type Manager struct {
    cfg     *config.RootConfig
    Modules []*module.Module
    errors  *errors.LintRuleErrorsList
    tracker *exclusions.ExclusionTracker
}

func NewManager(dir string, rootConfig *config.RootConfig) *Manager {
    managerLevel := pkg.Error
    m := &Manager{
        cfg: rootConfig,
        errors:  errors.NewLintRuleErrorsList().WithMaxLevel(&managerLevel),
        tracker: exclusions.NewExclusionTracker(),
    }
    // ...
}
```

### In Linters

Linters use the tracker to create tracked rules with module information:

```go
func (l *ContainerTracked) applyContainerRulesTracked(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
    moduleName := object.GetModuleName()
    
    // Create tracked rule for DNS Policy with module tracking
    dnsPolicyRule := exclusions.NewTrackedKindRuleForModule(
        l.cfg.ExcludeRules.DNSPolicy.Get(),
        l.tracker,
        ID,
        "dns-policy",
        moduleName,
    )

    // Apply rule with tracking
    if dnsPolicyRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
        // Perform check
    }
}
```

### Registering Rules Without Exclusions

For rules that don't have exclusions but should be tracked:

```go
// Register rules without exclusions in tracker
if l.tracker != nil {
    l.tracker.RegisterExclusionsForModule(ID, "recommended-labels", []string{}, moduleName)
    l.tracker.RegisterExclusionsForModule(ID, "namespace-labels", []string{}, moduleName)
}
```

### Handling Disabled Rules

For rules that can be completely disabled:

```go
// If the rule is disabled, register this as a used exclusion
if l.cfg.Conversions.Disable {
    l.tracker.RegisterExclusionsForModule(ID, "conversions", []string{}, moduleName)
} else {
    // If the rule is enabled, use exclusions for specific files
    trackedConversionsRule := exclusions.NewTrackedStringRuleForModule(
        l.cfg.ExcludeRules.Conversions.Files.Get(),
        l.tracker,
        ID,
        "conversions",
        moduleName,
    )
    rules.NewConversionsRuleTracked(trackedConversionsRule).CheckConversions(m.GetPath(), errorList)
}
```

### Output Results

At the end of execution, the manager outputs unused exclusions:

```go
func (m *Manager) PrintResult() {
    // Output linter errors
    // ...

    // Output unused exclusions as warnings
    unusedExclusions := m.tracker.FormatUnusedExclusions()
    if unusedExclusions != "" {
        fmt.Println(color.New(color.FgHiYellow).SprintFunc()("⚠️  WARNING: "))
        fmt.Println(color.New(color.FgHiYellow).SprintFunc()(unusedExclusions))
    }
}
```

## Example Output

```
⚠️  WARNING: 
Unused exclusions found:
  container:
    dns-policy:
      - StatefulSet/unused-statefulset (from modules: module1, module2)
      - Deployment/old-deployment (from modules: module1)
    security-context:
      - DaemonSet/legacy-daemonset (from modules: module3)
  openapi:
    enum:
      - old-schema.yaml (from modules: module4)
```

## API

### ExclusionTracker

```go
// Create new tracker
tracker := exclusions.NewExclusionTracker()

// Register exclusions (general)
tracker.RegisterExclusions(linterID, ruleID, exclusions)

// Register exclusions for specific module
tracker.RegisterExclusionsForModule(linterID, ruleID, exclusions, moduleName)

// Mark exclusion as used
tracker.MarkExclusionUsed(linterID, ruleID, exclusion)

// Get unused exclusions
unused := tracker.GetUnusedExclusions()

// Get usage statistics
stats := tracker.GetUsageStats()

// Format for output
formatted := tracker.FormatUnusedExclusions()
```

### Tracked Rules

```go
// Create tracked rules (general)
stringRule := exclusions.NewTrackedStringRule(excludeRules, tracker, linterID, ruleID)
kindRule := exclusions.NewTrackedKindRule(excludeRules, tracker, linterID, ruleID)
containerRule := exclusions.NewTrackedContainerRule(excludeRules, tracker, linterID, ruleID)

// Create tracked rules for specific modules
stringRule := exclusions.NewTrackedStringRuleForModule(excludeRules, tracker, linterID, ruleID, moduleName)
kindRule := exclusions.NewTrackedKindRuleForModule(excludeRules, tracker, linterID, ruleID, moduleName)
containerRule := exclusions.NewTrackedContainerRuleForModule(excludeRules, tracker, linterID, ruleID, moduleName)

// Use rules
if stringRule.Enabled("test-string") {
    // Perform check
}

if kindRule.Enabled("Deployment", "test-deployment") {
    // Perform check
}
```

## Available Tracked Rule Types

### String Rules
```go
// For string-based exclusions
trackedRule := exclusions.NewTrackedStringRuleForModule(
    excludeRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Prefix Rules
```go
// For prefix-based exclusions
trackedRule := exclusions.NewTrackedPrefixRuleForModule(
    excludeRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Kind Rules
```go
// For object type and name exclusions
trackedRule := exclusions.NewTrackedKindRuleForModule(
    excludeRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Container Rules
```go
// For container-specific exclusions
trackedRule := exclusions.NewTrackedContainerRuleForModule(
    excludeRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Service Port Rules
```go
// For service port exclusions
trackedRule := exclusions.NewTrackedServicePortRuleForModule(
    excludeRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Path Rules
```go
// For path-based exclusions (combines string and prefix rules)
trackedRule := exclusions.NewTrackedPathRuleForModule(
    excludeStringRules,
    excludePrefixRules,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

### Bool Rules
```go
// For boolean flags (disabled rules)
trackedRule := exclusions.NewTrackedBoolRuleForModule(
    disable,
    tracker,
    linterID,
    ruleID,
    moduleName,
)
```

## Testing

The system includes tests to verify correct operation:

```bash
go test ./pkg/exclusions -v
```

Tests cover:
- Basic exclusion tracking
- Working with multiple rules
- Module-specific tracking
- Cases without unused exclusions
- Usage statistics
- Integration with different rule types

## Migrating Existing Linters

To migrate existing linters to the new system:

1. Add tracker to linter constructor
2. Replace regular rules with tracked versions using `ForModule` constructors
3. Update rule application logic to use `Enabled()` methods
4. Register rules without exclusions using `RegisterExclusionsForModule()`
5. Handle disabled rules by registering them as used exclusions

See example migrations in:
- `pkg/linters/container/container.go`
- `pkg/linters/module/module.go`
- `pkg/linters/rbac/rbac.go`
- `pkg/linters/templates/templates.go` 