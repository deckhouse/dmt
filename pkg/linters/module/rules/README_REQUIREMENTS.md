# Requirements Architecture

## Overview

The requirements system has been refactored to support multiple component types and provide a flexible, extensible architecture for validating module requirements.

## Architecture Components

### 1. Component Types

The system supports different types of components that can have version requirements:

```go
type ComponentType string

const (
    ComponentDeckhouse ComponentType = "deckhouse"
    ComponentK8s       ComponentType = "kubernetes"
    ComponentModule    ComponentType = "module"
)
```

### 2. Component Requirements

Each requirement check can specify multiple component requirements:

```go
type ComponentRequirement struct {
    ComponentType ComponentType
    MinVersion    string
    Description   string
}
```

### 3. Requirement Checks

Requirement checks define what to validate and when:

```go
type RequirementCheck struct {
    Name         string
    Requirements []ComponentRequirement
    Description  string
    Detector     func(modulePath string, module *DeckhouseModule) bool
}
```

### 4. Requirements Registry

The registry manages all requirement checks and provides a centralized way to add new checks:

```go
type RequirementsRegistry struct {
    checks []RequirementCheck
}
```

## Current Requirements

### Stage Requirements

- **Trigger**: Module has a `stage` field
- **Requirement**: Deckhouse version >= 1.68.0
- **Component**: deckhouse

### Go Hooks Requirements

- **Trigger**: Module has Go hooks with module-sdk dependency and app.Run calls
- **Requirement**: Deckhouse version >= 1.68.0
- **Component**: deckhouse

### Readiness Probes Requirements

- **Trigger**: Module has readiness probes (app.WithReadiness) with module-sdk >= 0.3
- **Requirement**: Deckhouse version >= 1.71.0
- **Component**: deckhouse

## Module Configuration

The module.yaml file supports requirements for multiple components:

```yaml
name: my-module
namespace: my-namespace
stage: "General Availability"
requirements:
  deckhouse: ">= 1.68.0"
  kubernetes: ">= 1.28.0"
  modules:
    some-required-module: ">= 1.0.0"
```

## Extending the System

### Adding New Component Types

1. Add a new component type constant:

```go
const (
    ComponentNewFeature ComponentType = "new-feature"
)
```

2. Extend the `ModulePlatformRequirements` struct in `module_yaml.go`:

```go
type ModulePlatformRequirements struct {
    Deckhouse    string `json:"deckhouse,omitempty"`
    Kubernetes   string `json:"kubernetes,omitempty"`
    Bootstrapped bool   `json:"bootstrapped,omitempty"`
    NewFeature   string `json:"new-feature,omitempty"`  // New field
}
```

3. Add validation logic in `validateComponentRequirement`:

```go
case ComponentNewFeature:
    if module.Requirements.NewFeature == "" {
        errorList.Errorf("requirements [%s]: %s, new-feature version constraint is required", checkName, req.Description)
        return
    }
    constraintStr = module.Requirements.NewFeature
    constraintName = "new-feature"
```

### Adding New Requirement Checks

1. Create a detector function:

```go
func hasNewFeature(modulePath string, module *DeckhouseModule) bool {
    // Check if module uses the new feature
    // Return true if the feature is detected
    return false
}
```

2. Register the check in `NewRequirementsRegistry()`:

```go
registry.RegisterCheck(RequirementCheck{
    Name: "new_feature",
    Requirements: []ComponentRequirement{
        {
            ComponentType: ComponentDeckhouse,
            MinVersion:    "1.75.0",
            Description:   "New feature requires minimum Deckhouse version",
        },
        {
            ComponentType: ComponentK8s,
            MinVersion:    "1.28.0",
            Description:   "New feature requires minimum Kubernetes version",
        },
    },
    Description: "New feature usage requires minimum versions of multiple components",
    Detector: func(modulePath string, module *DeckhouseModule) bool {
        return hasNewFeature(modulePath, module)
    },
})
```

### Multiple Component Requirements

A single requirement check can validate multiple components:

```go
RequirementCheck{
    Name: "multi_component_feature",
    Requirements: []ComponentRequirement{
        {
            ComponentType: ComponentDeckhouse,
            MinVersion:    "1.75.0",
            Description:   "Feature requires minimum Deckhouse version",
        },
        {
            ComponentType: ComponentK8s,
            MinVersion:    "1.28.0",
            Description:   "Feature requires minimum Kubernetes version",
        },
    },
    // ...
}
```

## Error Messages

The system provides consistent error messages:

- **Missing requirements**: `"requirements [check_name]: description, component version range should start no lower than X.Y.Z"`
- **Invalid constraint**: `"requirements [check_name]: invalid component version constraint: constraint"`
- **Version too low**: `"requirements [check_name]: description, component version range should start no lower than X.Y.Z (currently: A.B.C)"`

## Testing

The system includes comprehensive tests for all requirement checks. To add tests for new requirements:

1. Create test cases in the appropriate test file
2. Use the `TestCase` structure for consistent test setup
3. Use helper functions from `requirements_test_helpers.go`

## Benefits of the New Architecture

1. **Extensibility**: Easy to add new component types and requirement checks
2. **Flexibility**: Support for multiple component requirements per check
3. **Consistency**: Unified error message format and validation logic
4. **Maintainability**: Clear separation of concerns and modular design
5. **Testability**: Comprehensive test coverage and helper utilities
6. **Future-proof**: Designed to support upcoming requirements (Kubernetes, module dependencies)

## Migration Notes

The new architecture maintains backward compatibility with existing module configurations. All existing tests continue to pass, and the public API remains unchanged.
