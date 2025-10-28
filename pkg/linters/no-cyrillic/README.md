# No-Cyrillic Linter

## Overview

The **No-Cyrillic Linter** validates that source code files don't contain cyrillic (Russian) characters. This linter helps maintain code consistency, prevents accidental language mixing in technical files, and ensures that code remains accessible to international developers who may not read cyrillic scripts.

Source code, configuration files, and technical documentation should use English for variable names, comments, keys, and values. The linter automatically scans YAML, JSON, and Go files to detect and report cyrillic characters.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [files](#files) | Validates source files don't contain cyrillic characters | ✅ | enabled |

## Rule Details

### files

**Purpose:** Ensures source code files remain cyrillic-free to maintain international accessibility, code consistency, and best practices. Cyrillic characters in code can cause issues with tooling, make code reviews difficult for non-Russian speakers, and violate coding standards.

**Description:**

The linter scans all source code files (YAML, YML, JSON, Go) and checks for cyrillic characters (А-Я, а-я, Ё, ё). When detected, it provides visual indicators showing exactly where the problematic characters appear.

**What it checks:**

1. Scans files with extensions: `.yaml`, `.yml`, `.json`, `.go`
2. Detects cyrillic characters (А-Я, а-я, Ё, ё) in file content
3. Reports line-by-line occurrences with visual pointers
4. Provides exact character positions for easy identification

**Automatically Skipped:**

The linter intelligently skips files where cyrillic is expected or acceptable:

- **Russian documentation**: Files matching patterns:
  - `doc-ru-*.yaml` or `doc-ru-*.yml` - Russian documentation files
  - `*_RU.md` - Russian markdown files
  - `*_ru.html` - Russian HTML files
  - Files in `docs/site/` or `docs/documentation/`
  - Files in `tools/spelling/`

- **Conversion descriptions**: 
  - `openapi/conversions/*.yaml` - Conversion descriptions include Russian translations

- **Module definitions**:
  - `module.yaml` - Contains Russian descriptions and labels

- **Internationalization**:
  - Files in `/i18n/` directories - Translation files

- **Linter itself**:
  - `no_cyrillic.go` and `no_cyrillic_test.go` - The linter's own code

**Why it matters:**

1. **International Collaboration**: Code with cyrillic characters is inaccessible to developers who don't read Russian
2. **Tooling Compatibility**: Some development tools and CI/CD systems have issues with non-ASCII characters
3. **Code Review**: Mixed-language code makes reviews harder and error-prone
4. **Best Practices**: Industry standards require English for code
5. **Search and Indexing**: Search tools may not properly index cyrillic characters

**Examples:**

❌ **Incorrect** - Cyrillic in YAML configuration:

```yaml
# config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  # Настройки приложения
  app_name: "my-app"
  логирование: "enabled"  # Cyrillic key
  description: "Описание модуля"  # Cyrillic value
```

**Error:**
```
Error: has cyrillic letters
File: config.yaml
Value:
  # Настройки приложения
    ^^^^^^^^^^
  логирование: "enabled"
  ^^^^^^^^^^^^
  description: "Описание модуля"
               ^^^^^^^^^^^^^^^
```

❌ **Incorrect** - Cyrillic in Go code:

```go
// module.go
package main

// Инициализация модуля
func initialize() {
    // Загрузка конфигурации
    config := loadConfig()
    
    имяМодуля := "my-module"  // Cyrillic variable name
}
```

**Error:**
```
Error: has cyrillic letters
File: module.go
Value:
// Инициализация модуля
   ^^^^^^^^^^^^^^^
    // Загрузка конфигурации
       ^^^^^^^^
    имяМодуля := "my-module"
    ^^^^^^^^^
```

❌ **Incorrect** - Cyrillic in JSON:

```json
{
  "name": "my-module",
  "описание": "Module description",
  "version": "1.0.0",
  "настройки": {
    "enabled": true
  }
}
```

**Error:**
```
Error: has cyrillic letters
File: config.json
Value:
  "описание": "Module description",
   ^^^^^^^^
  "настройки": {
   ^^^^^^^^^
```

✅ **Correct** - English-only YAML:

```yaml
# config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  # Application settings
  app_name: "my-app"
  logging: "enabled"
  description: "Module description"
```

✅ **Correct** - English-only Go code:

```go
// module.go
package main

// Initialize module
func initialize() {
    // Load configuration
    config := loadConfig()
    
    moduleName := "my-module"
}
```

✅ **Correct** - English-only JSON:

```json
{
  "name": "my-module",
  "description": "Module description",
  "version": "1.0.0",
  "settings": {
    "enabled": true
  }
}
```

✅ **Correct** - Cyrillic in allowed files:

```yaml
# module.yaml (automatically skipped)
name: my-module
description: Module description
descriptionRu: Описание модуля  # Cyrillic allowed here
```

```markdown
<!-- README_RU.md (automatically skipped) -->
# Мой Модуль

Это описание модуля на русском языке.
```

```yaml
# openapi/conversions/v2.yaml (automatically skipped)
version: 2
description:
  en: "Migrated settings"
  ru: "Перенесены настройки"  # Cyrillic allowed in conversions
```

**Visual Error Format:**

The linter provides precise character-level indicators to help you locate cyrillic characters:

```
Line content: Check the документация for details
Visual pointer:             ^^^^^^^^^^^^
              --------------^^^^^^^^^^^^^---------
```

Each `^` points to a cyrillic character, making it easy to find and replace them.

**Configuration:**

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    impact: error  # error | warning | info | ignored
```

To exclude specific files:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    exclude-rules:
      files:
        - path/to/legacy-file.go
        - config/old-config.yaml
```

To exclude entire directories:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    exclude-rules:
      directories:
        - vendor/
        - third-party/
        - legacy-code/
```

Combined example:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    impact: error
    exclude-rules:
      files:
        - images/legacy/script.sh
        - hooks/migration.go
      directories:
        - vendor/
        - third-party/russian-library/
```

## Configuration

The No-Cyrillic linter can be configured at the module level with path-based exclusions.

### Module-Level Settings

Configure the overall impact level for the no-cyrillic linter:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### File Exclusions

Exclude specific files from cyrillic checking:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    exclude-rules:
      files:
        - path/to/file.go
        - path/to/config.yaml
        - scripts/legacy-script.sh
```

**Pattern matching:**
- Paths are relative to the module root
- Exact file path matching
- Supports any file extension

### Directory Exclusions

Exclude entire directories from cyrillic checking:

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    exclude-rules:
      directories:
        - vendor/
        - third-party/
        - legacy/
        - external/russian-lib/
```

**Pattern matching:**
- Paths are relative to the module root
- Directory prefix matching
- All files within excluded directories are skipped

### Complete Configuration Example

```yaml
# .dmt.yaml
linters-settings:
  no-cyrillic:
    # Global impact level
    impact: error
    
    # Exclusion rules
    exclude-rules:
      # Specific files to exclude
      files:
        - images/legacy/old-script.sh
        - hooks/migration/convert.go
        - templates/legacy-template.yaml
      
      # Directories to exclude
      directories:
        - vendor/
        - third-party/
        - legacy-code/
        - external/russian-library/
        - tools/migration/
```

### Configuration in Module Directory

You can also place a `.dmt.yaml` configuration file directly in your module directory:

```yaml
# modules/my-module/.dmt.yaml
linters-settings:
  no-cyrillic:
    impact: warning  # More lenient for this specific module
    exclude-rules:
      files:
        - hooks/legacy-hook.go
      directories:
        - third-party/
```
