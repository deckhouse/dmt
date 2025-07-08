# Exclusion Tracking System

The exclusion tracking system allows determining which configured exclusions are not used during linter operation and outputs them in the final log as warnings.

## Description

In the DMT project, a configuration is implemented that presents exclusions for linters. When running, these exclusions are used to determine which errors to include in the result log and which ones to skip.

The new functionality allows:
- Tracking the usage of each exclusion during linter operation
- Determining which exclusions were not used
- Outputting a list of unused exclusions in the final log as warnings

## Architecture

### ExclusionTracker

The main component of the system is `ExclusionTracker`, which:
- Registers all configured exclusions for each linter and rule
- Tracks exclusion usage during execution
- Provides methods for getting usage statistics

### Tracked Rules

Extended versions of rules have been created that inherit from base rules and add tracking functionality:
- `TrackedStringRule` - for string exclusions
- `TrackedPrefixRule` - for prefix-based exclusions
- `TrackedKindRule` - for object type and name exclusions
- `TrackedContainerRule` - for container exclusions
- `TrackedServicePortRule` - for service port exclusions
- `TrackedPathRule` - for path-based exclusions
- `TrackedBoolRule` - for boolean exclusions

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

Linters use the tracker to create tracked rules:

```go
func (l *ContainerTracked) applyContainerRulesTracked(object storage.StoreObject, errorList *errors.LintRuleErrorsList) {
    // Create tracked rule for DNS Policy
    dnsPolicyRule := exclusions.NewTrackedKindRule(
        l.cfg.ExcludeRules.DNSPolicy.Get(),
        l.tracker,
        ID,
        "dns-policy",
    )

    // Apply rule with tracking
    if dnsPolicyRule.Enabled(obj.Unstructured.GetKind(), obj.Unstructured.GetName()) {
        // Perform check
    }
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
      - StatefulSet/unused-statefulset
      - Deployment/old-deployment
    security-context:
      - DaemonSet/legacy-daemonset
  openapi:
    enum:
      - old-schema.yaml
```

## API

### ExclusionTracker

```go
// Create new tracker
tracker := exclusions.NewExclusionTracker()

// Register exclusions
tracker.RegisterExclusions(linterID, ruleID, exclusions)

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
// Create tracked rules
stringRule := exclusions.NewTrackedStringRule(excludeRules, tracker, linterID, ruleID)
kindRule := exclusions.NewTrackedKindRule(excludeRules, tracker, linterID, ruleID)
containerRule := exclusions.NewTrackedContainerRule(excludeRules, tracker, linterID, ruleID)

// Use rules
if stringRule.Enabled("test-string") {
    // Perform check
}

if kindRule.Enabled("Deployment", "test-deployment") {
    // Perform check
}
```

## Testing

The system includes tests to verify correct operation:

```bash
go test ./pkg/exclusions -v
```

Tests cover:
- Basic exclusion tracking
- Working with multiple rules
- Cases without unused exclusions
- Usage statistics

## Migrating Existing Linters

To migrate existing linters to the new system:

1. Add tracker to linter constructor
2. Replace regular rules with tracked versions
3. Update rule application logic to use `Enabled()` methods

See example migration for container linter in file `pkg/linters/container/container_tracked.go`. 