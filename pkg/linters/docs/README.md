# Documentation Linter

## Overview

The **Documentation Linter** validates module documentation to ensure proper structure, completeness, and language consistency. This linter enforces bilingual documentation requirements, checks for documentation file presence, validates that English documentation doesn't contain cyrillic characters, and ensures markdown files follow deckhouse markdown style conventions.

Proper documentation is critical for Deckhouse modules as it helps users understand module features, configuration options, and usage patterns. The linter ensures documentation meets quality standards and is accessible to both English and Russian-speaking audiences.

## Rules

| Rule | Description | Configurable | Default |
|------|-------------|--------------|---------|
| [readme](#readme) | Validates presence of README.md in docs/ directory | ✅ | enabled |
| [bilingual](#bilingual) | Validates documentation exists in both English and Russian | ✅ | enabled |
| [cyrillic-in-english](#cyrillic-in-english) | Validates English documentation doesn't contain cyrillic characters | ✅ | enabled |
| [no-lang-key](#no-lang-key) | Validates documentation front matter doesn't contain `lang` key | ✅ | enabled |
| [markdownlint](#markdownlint) | Validates markdown files in docs/ follow deckhouse markdown style | ✅ | enabled |

"Configurable" means that this rule can be configured using the `.dmtlint.yaml` file, including customizing the rule's parameters and/or disabling the rule.

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
# .dmtlint.yaml
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
# .dmtlint.yaml
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
# .dmtlint.yaml
linters-settings:
  documentation:
    rules:
      cyrillic-in-english:
        exclude:
          - docs/GLOSSARY.md      # Contains technical terms in multiple languages
```

### no-lang-key

**Purpose:** Ensures that documentation files don't contain the `lang` key in their YAML front matter. The language of the document should be determined by its file name convention (e.g., `.ru.md` suffix for Russian) rather than by a `lang` field in the front matter.

**Description:**

Markdown documentation files in `docs/` should not include a `lang:` key in their YAML front matter block. The language is already encoded in the file naming convention (`.md` for English, `.ru.md` for Russian), so an additional `lang` field is redundant and can cause inconsistencies.

**What it checks:**

1. Scans all `.md` files in `docs/` directory (top-level only)
2. Extracts the YAML front matter (content between the first pair of `---` delimiters)
3. Checks for the presence of a `lang:` key in the front matter
4. Reports the exact line number where the `lang` key is found

**Why it matters:**

Having a `lang` key in the front matter is redundant because the documentation system already determines the language from the file name suffix. It can also lead to inconsistencies if the `lang` value doesn't match the actual file language. Removing it simplifies the documentation structure and prevents potential mismatches.

**Examples:**

:x: **Incorrect** - Front matter with `lang` key:

```markdown
---
title: "Module dashboard"
lang: ru
description: "Web interface for Kubernetes Dashboard."
---

## Authentication
...
```

**Error:**
```
Documentation contains 'lang' key in front matter; this field should be removed
File: docs/README.ru.md
Line 3: front matter contains 'lang' key which should be removed
```

:x: **Incorrect** - Front matter with `lang` key and additional metadata:

```markdown
---
title: "Module dashboard"
lang: ru
description: "Web interface."
webIfaces:
- name: dashboard
---
```

:white_check_mark: **Correct** - Front matter without `lang` key:

```markdown
---
title: "Module dashboard"
description: "Web interface for Kubernetes Dashboard."
---

## Authentication
...
```

:white_check_mark: **Correct** - Front matter without `lang` key and with additional metadata:

```markdown
---
title: "Module dashboard"
description: "Web interface."
webIfaces:
- name: dashboard
---
```

**Configuration:**

To exclude specific files from this check:
```yaml
# .dmtlint.yaml
linters-settings:
  documentation:
    rules:
      no-lang-key:
        exclude:
          - docs/LEGACY.md      # Legacy file that still uses lang key
```

---

### markdownlint

**Purpose:** Ensures markdown files in the `docs/` directory follow consistent deckhouse markdown style conventions (headings, lists, code blocks, etc.).

**Description:**

This rule runs the [go-markdownlint](https://github.com/ldmonster/go-markdownlint) library against every `.md` file under `docs/` (recursively, including `docs/internal/...`) and reports any markdown style violations. The built-in rule set is enabled by default; only a fixed set of deckhouse-specific overrides is applied (line-length limits, blanks-around-headings, duplicate-heading siblings, etc.).

Unlike the other documentation rules, `markdownlint` reports at `warn` **by default** — its findings are shown but do not fail the run. Set `impact: error` to make violations fatal.

**What it checks:**

1. Recursively scans all `.md` files under `docs/` (top-level and nested, e.g. `docs/internal/`)
2. Lints each file with the built-in markdownlint rules using the deckhouse configuration overrides
3. Reports the rule name(s), description, file path and line number for each violation

**Why it matters:**

Consistent markdown style across all modules makes the documentation easier to read, review and maintain, and keeps it aligned with the rest of the deckhouse documentation.

**Rule reference:**

Findings are reported as `MDxxx/rule-name …`. Look up the code below to see what it means. Rules marked *(tuned)* use deckhouse-specific settings.

| Rule (as shown in the error) | What it means |
|------------------------------|---------------|
| MD001 / heading-increment | Heading levels must increase one at a time — no jump from `#` to `###`. |
| MD003 / heading-style | Heading style must be consistent (ATX `#`, not closed `# … #` or setext). |
| MD005 / list-indent | List items at the same level must share the same indentation. |
| MD007 / ul-indent | Nested bullet lists must be indented by the expected amount. |
| MD009 / no-trailing-spaces | No trailing spaces at the end of a line. |
| MD010 / no-hard-tabs | No hard tabs — use spaces. |
| MD011 / no-reversed-links | Reversed link syntax `(text)[url]` instead of `[text](url)`. |
| MD012 / no-multiple-blanks | No multiple consecutive blank lines. |
| MD013 / line-length *(tuned)* | Line too long. Limits: 1000 chars (headings 128, code blocks 400). |
| MD014 / commands-show-output | `$` before shell commands only when their output is shown. |
| MD018 / no-missing-space-atx | Space required after `#` in a heading (`# Title`, not `#Title`). |
| MD019 / no-multiple-space-atx | At most one space after `#` in a heading. |
| MD020 / no-missing-space-closed-atx | Space required inside a closed heading `# Title #`. |
| MD021 / no-multiple-space-closed-atx | At most one space inside a closed heading. |
| MD022 / blanks-around-headings *(tuned)* | Headings must be surrounded by blank lines (1 above, 1 below). |
| MD023 / heading-start-left | Headings must start at the beginning of the line (no indent). |
| MD024 / no-duplicate-heading *(tuned)* | No duplicate heading text — checked among sibling headings only. |
| MD025 / single-title / single-h1 | Only one top-level (`#`) heading per document. |
| MD026 / no-trailing-punctuation *(tuned)* | No trailing punctuation in headings (`. , ; : !` and CJK variants). |
| MD027 / no-multiple-space-blockquote | At most one space after `>` in a blockquote. |
| MD028 / no-blanks-blockquote | No blank line inside a blockquote (it splits it in two). |
| MD029 / ol-prefix *(tuned)* | Ordered-list numbering — all `1.` or strictly ascending (`one_or_ordered`). |
| MD030 / list-marker-space | Correct number of spaces after a list marker. |
| MD031 / blanks-around-fences | Fenced code blocks must be surrounded by blank lines. |
| MD034 / no-bare-urls | Bare URLs must be wrapped in `<…>` or `[text](url)`. |
| MD035 / hr-style | Horizontal-rule style must be consistent (e.g. always `---`). |
| MD036 / no-emphasis-as-heading | Don't use bold/italic text in place of a heading. |
| MD037 / no-space-in-emphasis | No spaces inside emphasis markers (`**bold**`, not `** bold **`). |
| MD038 / no-space-in-code | No spaces inside inline code (`` `code` ``, not `` ` code ` ``). |
| MD039 / no-space-in-links | No spaces inside link text (`[link]`, not `[ link ]`). |
| MD040 / fenced-code-language | Fenced code blocks must declare a language after the opening fence (e.g. `yaml`, `bash`). |
| MD041 / first-line-heading / first-line-h1 *(tuned)* | First line must be a top-level heading (front-matter `title` counts). |
| MD042 / no-empty-links | No empty links (`[text]()`). |
| MD045 / no-alt-text | Images must have alt text (`![alt](img.png)`). |
| MD046 / code-block-style | Code-block style must be consistent within a file (fenced vs indented). |
| MD047 / single-trailing-newline | File must end with exactly one newline. |
| MD048 / code-fence-style | Code-fence style must be consistent (all fences use backticks, or all use tildes `~~~`). |
| MD049 / emphasis-style | Italic style must be consistent (`*` or `_`). |
| MD050 / strong-style | Bold style must be consistent (`**` or `__`). |
| MD052 / reference-links-images | Reference links/images must point to a defined label. |
| MD053 / link-image-reference-definitions | Reference definitions (`[label]: url`) must be used — no unused ones. |
| MD055 / table-pipe-style | Table leading/trailing pipe (`\|`) style must be consistent. |
| MD056 / table-column-count | Every table row must have the same number of columns. |
| MD058 / blanks-around-tables | Tables must be surrounded by blank lines. |
| MD059 / descriptive-link-text | Link text must be descriptive — not `here`, `link`, `click here`. |

Three rules are enabled but effectively inert under the deckhouse config, so you will not see them fire: **MD043** (required-headings — no required structure is set), **MD044** (proper-names — the name list is empty) and **MD054** (link-image-style — all link/image styles are allowed by default).

Rules **disabled** on purpose (never reported): MD002 (first-heading-h1, deprecated), MD004 (ul-style), MD032 (blanks-around-lists), MD033 (no-inline-html — HTML is allowed), MD051 (link-fragments — Deckhouse anchors only exist after the doc build), MD060 (table-column-style).

**Examples:**

❌ **Incorrect** - Duplicate top-level heading (MD025) and missing trailing newline (MD047):

```markdown
<!-- docs/README.md -->
# My Module

# My Module
```

(file has no trailing newline)

**Error:**
```
MD025/single-title/single-h1 Multiple top-level headings in the same document
File: docs/README.md
Line: 3

MD047/single-trailing-newline Files should end with a single newline character
File: docs/README.md
Line: 3
```

✅ **Correct** - Single top-level heading and trailing newline:

```markdown
<!-- docs/README.md -->
# My Module
```

**Configuration:**

To make this rule fatal, or to disable it:

```yaml
# .dmtlint.yaml
linters-settings:
  documentation:
    rules:
      markdownlint:
        impact: error  # fail the run on violations (default is warn)
        # impact: ignored   # disable the rule entirely
```

---

## Configuration

The Documentation linter can be configured at both the module level and for individual rules.

### Module-Level Settings

Configure the overall impact level for the documentation linter:
```yaml
# .dmtlint.yaml
linters-settings:
  documentation:
    impact: error  # Options: error, warning, info, ignored
```

**Impact levels:**
- `error`: Violations fail the validation and return a non-zero exit code
- `warning`: Violations are reported but don't fail the validation
- `info`: Violations are reported as informational messages
- `ignored`: The linter is completely disabled

### Path-Based Exclusions

Exclude specific modules or files from validation:
```yaml
# .dmtlint.yaml
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
   # .dmtlint.yaml
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

