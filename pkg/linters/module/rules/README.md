# Requirements Rules Tests

This directory contains tests for the requirements validation rules in the DMT module linter.

## Test Structure

The tests have been refactored to improve maintainability and readability by splitting the original 855-line test file into smaller, focused test files:

### Test Files

- **`requirements_test_helpers.go`** - Common test utilities and helper functions
- **`requirements_edge_cases_test.go`** - Comprehensive tests for requirements validation including edge cases
- **`module_yaml_test.go`** - Tests for module.yaml parsing and validation
- **`license_library_test.go`** - Tests for license library validation
- **`oss_library_test.go`** - Tests for OSS library validation

### Test Helper Structure

The `TestHelper` struct provides common utilities:

```go
type TestHelper struct {
    t *testing.T
}
```

#### Key Methods

- `CreateTempModule(name string) string` - Creates a temporary module directory
- `SetupModule(modulePath, content string)` - Creates module.yaml with given content
- `SetupGoHooks(modulePath, goModContent, mainGoContent string)` - Sets up Go hooks files
- `RunRequirementsCheck(modulePath string) *errors.LintRuleErrorsList` - Runs the requirements check
- `AssertErrors(errorList *errors.LintRuleErrorsList, expectedErrors []string)` - Asserts expected errors
- `RunTestCase(tc TestCase)` - Runs a complete test case

### Test Case Structure

```go
type TestCase struct {
    Name           string
    Setup          TestSetup
    ExpectedErrors []string
    Description    string
}

type TestSetup struct {
    ModuleContent string
    SetupFiles    func(string) error
}
```

### Common Test Data

Constants are defined for common test content:

- `ValidModuleContent` - Basic valid module.yaml content
- `StageModuleContent` - Module with stage field
- `StageWithRequirementsContent` - Module with stage and valid requirements
- `GoModWithModuleSDK` - go.mod with module-sdk v0.1.0
- `GoModWithModuleSDK03` - go.mod with module-sdk v0.3.0
- `MainGoWithAppRun` - main.go with app.Run() call
- `MainGoWithReadiness` - main.go with app.WithReadiness() call
- `MainGoEmpty` - Empty main.go

## Test Categories

### 1. Basic Functionality Tests
- Rule creation and initialization
- Version constraint parsing and validation
- Module file loading and parsing
- Constants validation

### 2. Stage Requirements Tests
- Stage field detection
- Minimum Deckhouse version validation (1.68.0)
- Various constraint formats (>=, >, =, ranges)
- Error handling for invalid constraints

### 3. Go Hooks Tests
- Detection of Go hooks (go.mod + module-sdk + app.Run)
- Requirements validation for Go hooks (1.68.0 minimum)
- Edge cases (no module-sdk, no Run calls)

### 4. Readiness Probes Tests
- Detection of readiness probes (module-sdk >= 0.3 + app.WithReadiness)
- Requirements validation for readiness probes (1.71.0 minimum)
- Version-specific behavior

### 5. Integration Tests
- Combined scenarios with multiple requirements
- End-to-end validation workflows
- User requirement scenarios

## Running Tests

```bash
# Run all requirements tests
go test ./pkg/linters/module/rules/... -v

# Run specific test file
go test ./pkg/linters/module/rules/requirements_edge_cases_test.go -v

# Run with coverage
go test ./pkg/linters/module/rules/... -cover
```

## Benefits of Refactoring

1. **Improved Readability** - Each test file focuses on a specific aspect
2. **Better Maintainability** - Easier to find and modify specific tests
3. **Reduced Duplication** - Common setup logic extracted to helpers
4. **Clearer Test Intent** - Test names and structure better reflect what's being tested
5. **Faster Development** - Easier to add new tests without navigating large files
6. **Better Organization** - Logical grouping of related test cases

## Adding New Tests

When adding new tests:

1. Use the existing `TestHelper` methods for common operations
2. Follow the `TestCase` structure for consistent test organization
3. Add new constants to the common test data section if needed
4. Place tests in the appropriate file based on functionality
5. Use descriptive test names that explain the scenario being tested 