# Documentation Linter

## Overview

The **Documentation Linter** validates module documentation to ensure proper structure, completeness, and language consistency. This linter enforces bilingual documentation requirements, checks for documentation file presence, and validates that English documentation doesn't contain cyrillic characters.

Proper documentation is critical for Deckhouse modules as it helps users understand module features, configuration options, and usage patterns. The linter ensures documentation meets quality standards and is accessible to both English and Russian-speaking audiences.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [readme](#readme) | Validates presence of README.md in docs/ directory | ✅ | enabled |
| [bilingual](#bilingual) | Validates documentation exists in both English and Russian | ✅ | enabled |
| [cyrillic-in-english](#cyrillic-in-english) | Validates English documentation doesn't contain cyrillic characters | ✅ | enabled |

## Rule Details

### readme

**Purpose:** Ensures every module has a primary documentation entry point in the `docs/README.md` file. This provides a consistent location for users to find module information and prevents modules from being deployed without documentation.

**Description:**

Every Deckhouse module must have a `docs/README.md` file that serves as the main documentation entry point. This file should contain an overview of the module, its features, configuration options, and usage examples.

**What it checks:**

1. Verifies that `docs/README.md` file exists in the module directory
2. Checks that the README.md file is not empty (size > 0 bytes)
3. Validates file is readable and accessible

**Why it matters:**

Documentation is essential for module adoption and proper usage. Without a README.md file, users won't understand what the module does, how to configure it, or how to troubleshoot issues. This rule ensures every module meets minimum documentation standards.

**Examples:**

❌ **Incorrect** - Missing README.md:

```
my-module/
├── templates/
│   └── deployment.yaml
├── openapi/
│   └── config-values.yaml
└── docs/
    └── CONFIGURATION.md          # Other docs exist but no README.md
```

**Error:**
```
README.md file is missing in docs/ directory
```

❌ **Incorrect** - Empty README.md:

```
my-module/
└── docs/
    └── README.md                 # File exists but is empty (0 bytes)
```

**Error:**
```
README.md file is empty
```

✅ **Correct** - Proper README.md:

```
my-module/
└── docs/
    └── README.md                 # Contains module documentation
```

```markdown
# My Module

## Overview
This module provides...

## Configuration
...

## Usage
...
```

**Configuration:**

To disable this rule for specific modules:

```yaml
# .dmt.yaml
linters-settings:
  documentation:
    rules:
      readme:
        exclude:
          - my-legacy-module
```

---

### bilingual

**Purpose:** Ensures module documentation is accessible to both English and Russian-speaking audiences by requiring documentation files in both languages. This maintains Deckhouse's commitment to bilingual support and helps users in different regions.

**Description:**

For every English documentation file in the `docs/` directory, a corresponding Russian translation must exist. Russian documentation files should use the `.ru.md` suffix (e.g., `README.ru.md` for `README.md`).

**What it checks:**

1. Scans all `.md` files in the `docs/` directory (top-level only)
2. For each English documentation file, checks for a corresponding `.ru.md` or `_RU.md` file
3. Validates that Russian counterparts exist for all documentation files
4. Ignores files that are already Russian (ending in `.ru.md` or `_RU.md`)

**Why it matters:**

Deckhouse is used by organizations globally, with significant adoption in Russian-speaking regions. Bilingual documentation ensures all users can effectively use and configure modules regardless of their language preference. Missing translations create accessibility barriers and reduce module adoption.

**Examples:**

❌ **Incorrect** - Missing Russian translation:

```
my-module/
└── docs/
    ├── README.md                 # English version exists
    ├── CONFIGURATION.md          # English version exists
    └── CONFIGURATION.ru.md       # Russian version exists for CONFIGURATION
                                  # ❌ Missing README.ru.md
```

**Error:**
```
Russian counterpart is missing: need to create a matching .ru.md in docs/
File: docs/README.md
```

✅ **Correct** - Complete bilingual documentation:

```
my-module/
└── docs/
    ├── README.md                 # English version
    ├── README.ru.md              # Russian translation
    ├── CONFIGURATION.md          # English version
    └── CONFIGURATION.ru.md       # Russian translation
```

✅ **Correct** - Legacy naming (still supported):

```
my-module/
└── docs/
    ├── README.md                 # English version
    ├── README_RU.md              # Russian translation (legacy format)
    ├── CONFIGURATION.md
    └── CONFIGURATION_RU.md
```

**Supported file naming conventions:**

- **Preferred:** `FILENAME.ru.md` (e.g., `README.ru.md`, `FAQ.ru.md`)
- **Legacy:** `FILENAME_RU.md` (e.g., `README_RU.md`, `FAQ_RU.md`) - case insensitive

**Configuration:**

To disable bilingual checks for specific files:

```yaml
# .dmt.yaml
linters-settings:
  documentation:
    rules:
      bilingual:
        exclude:
          - docs/INTERNAL.md      # Internal doc, no translation needed
```

---

### cyrillic-in-english

**Purpose:** Ensures English documentation files contain only English text and don't accidentally include cyrillic (Russian) characters. This maintains documentation quality, prevents language mixing, and ensures proper content organization.

**Description:**

English documentation files must not contain cyrillic characters. This rule scans all English `.md` files in the `docs/` directory and reports any lines containing Russian letters. This helps catch copy-paste errors, ensures proper language separation, and maintains documentation professionalism.

**What it checks:**

1. Scans all `.md` and `.markdown` files in `docs/` directory (top-level only)
2. Skips files ending in `.ru.md` or `_RU.md` (Russian documentation)
3. Detects cyrillic characters (А-Я, а-я, Ё, ё) in each line
4. Reports exact line numbers and positions where cyrillic characters appear
5. Provides visual indicators pointing to problematic characters

**Why it matters:**

Mixed-language documentation is confusing and unprofessional. Users reading English documentation shouldn't encounter Russian text, as it disrupts reading flow and may be incomprehensible to non-Russian speakers. This rule ensures clean language separation and high documentation quality.

**Examples:**

❌ **Incorrect** - Cyrillic in English documentation:

```markdown
<!-- docs/README.md -->
# My Module

This module provides мониторинг and logging features.

## Configuration

Use the настройки section to configure the module.
```

**Error:**
```
English documentation contains cyrillic characters
File: docs/README.md
Line 3: This module provides мониторинг and logging features.
                              ^^^^^^^^^^
Line 7: Use the настройки section to configure the module.
                ^^^^^^^^
```

❌ **Incorrect** - Copy-paste from Russian documentation:

```markdown
<!-- docs/FAQ.md -->
# FAQ

Q: How to install?
A: Следуйте инструкциям в разделе установки.
```

**Error:**
```
English documentation contains cyrillic characters
File: docs/FAQ.md
Line 5: A: Следуйте инструкциям в разделе установки.
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
```

✅ **Correct** - Clean English documentation:

```markdown
<!-- docs/README.md -->
# My Module

This module provides monitoring and logging features.

## Configuration

Use the settings section to configure the module.
```

✅ **Correct** - Russian content in Russian file:

```markdown
<!-- docs/README.ru.md -->
# Мой модуль

Этот модуль предоставляет возможности мониторинга и логирования.

## Конфигурация

Используйте раздел настроек для конфигурации модуля.
```

**Visual error format:**

The linter provides precise character-level indicators:

```
Line 42: Check the документация for more details.
                    ^^^^^^^^^^^^
         ----------------^^^^^^^-----------------
```

**Configuration:**

To exclude specific files from this check:

```yaml
# .dmt.yaml
linters-settings:
  documentation:
    rules:
      cyrillic-in-english:
        exclude:
          - docs/GLOSSARY.md      # Contains technical terms in multiple languages
```

## Configuration

### Path-Based Exclusions

Exclude specific modules or files from validation:

```yaml
# .dmt.yaml
linters-settings:
  documentation:
    rules:
      readme:
        exclude:
          - legacy-module       # Exclude entire module
      bilingual:
        exclude:
          - docs/INTERNAL.md    # Exclude specific file
      cyrillic-in-english:
        exclude:
          - docs/GLOSSARY.md    # Technical terms document
```

## Common Issues

### Issue: Missing README.md

**Symptom:**
```
Error: README.md file is missing in docs/ directory
```

**Cause:** The module doesn't have a `docs/README.md` file.

**Solutions:**

1. **Create the README.md file:**

   ```bash
   mkdir -p modules/my-module/docs
   cat > modules/my-module/docs/README.md << 'EOF'
   # My Module
   
   ## Overview
   Brief description of what this module does.
   
   ## Configuration
   Configuration options and examples.
   
   ## Usage
   How to use this module.
   EOF
   ```

2. **Use a documentation template:**

   ```bash
   # Copy from another module
   cp modules/reference-module/docs/README.md modules/my-module/docs/README.md
   # Then customize the content
   ```

### Issue: Missing Russian translation

**Symptom:**
```
Error: Russian counterpart is missing: need to create a matching .ru.md in docs/
File: docs/CONFIGURATION.md
```

**Cause:** An English documentation file exists without a corresponding Russian translation.

**Solutions:**

1. **Create the Russian translation:**

   ```bash
   # Create matching .ru.md file
   touch modules/my-module/docs/CONFIGURATION.ru.md
   ```

2. **Translate the content:**

   ```bash
   # Start with copying the English version
   cp modules/my-module/docs/CONFIGURATION.md \
      modules/my-module/docs/CONFIGURATION.ru.md
   # Then translate the content to Russian
   ```

3. **Exclude if translation is not needed (not recommended):**

   ```yaml
   # .dmt.yaml
   linters-settings:
     documentation:
       rules:
         bilingual:
           exclude:
             - docs/CONFIGURATION.md
   ```

### Issue: Cyrillic characters in English documentation

**Symptom:**
```
Error: English documentation contains cyrillic characters
File: docs/README.md
Line 15: Check the документация for details.
                    ^^^^^^^^^^^^
```

**Cause:** Russian text was accidentally included in an English documentation file, often from copy-paste operations.

**Solutions:**

1. **Replace cyrillic text with English:**

   ```markdown
   <!-- Before -->
   Check the документация for details.
   
   <!-- After -->
   Check the documentation for details.
   ```

2. **Review the entire document:**

   ```bash
   # Search for cyrillic characters
   grep -n '[А-Яа-яЁё]' modules/my-module/docs/README.md
   ```

3. **Use proper language files:**

   If you meant to write in Russian, use the Russian documentation file:
   ```markdown
   <!-- docs/README.ru.md -->
   Проверьте документацию для получения подробностей.
   ```

### Issue: Empty README.md file

**Symptom:**
```
Error: README.md file is empty
```

**Cause:** The `docs/README.md` file exists but contains no content (0 bytes).

**Solutions:**

1. **Add content to the file:**

   ```bash
   cat > modules/my-module/docs/README.md << 'EOF'
   # My Module
   
   [Add module description here]
   EOF
   ```

2. **Use a minimal template:**

   ```markdown
   # Module Name
   
   ## Description
   Brief description of the module functionality.
   
   ## Configuration
   See the OpenAPI schema for configuration options.
   ```

### Issue: Wrong file naming convention

**Symptom:**
```
Error: Russian counterpart is missing: need to create a matching .ru.md in docs/
File: docs/README.md
```

But you have `docs/README-RU.md` or `docs/README_ru.md` (lowercase).

**Cause:** The Russian file uses an incorrect naming convention.

**Solutions:**

1. **Rename to the correct format:**

   ```bash
   # Preferred format
   mv modules/my-module/docs/README-RU.md \
      modules/my-module/docs/README.ru.md
   
   # Legacy format (also acceptable)
   mv modules/my-module/docs/README-RU.md \
      modules/my-module/docs/README_RU.md
   ```

2. **Supported naming conventions:**
   - ✅ `FILENAME.ru.md` (preferred)
   - ✅ `FILENAME_RU.md` (legacy, case insensitive)
   - ❌ `FILENAME-RU.md` (not supported)

