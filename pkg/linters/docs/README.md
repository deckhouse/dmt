# Documentation Linter

Documentation linter checks module documentation requirements including:

## Rules

### README Rule
- **Name**: `readme`
- **Description**: Checks that module has README.md file
- **Details**: Ensures each module has proper documentation entry point

### Bilingual Rule  
- **Name**: `bilingual`
- **Description**: Checks that documentation exists in both English and Russian
- **Details**: Verifies that modules have documentation in both languages (README.md and README_RU.md)

### Cyrillic in English Rule
- **Name**: `cyrillic-in-english` 
- **Description**: Checks for cyrillic characters in English documentation files
- **Details**: Ensures English documentation (README.md, *.md files without _RU suffix) doesn't contain cyrillic characters

## Impact Levels

- `error` - Fails the linting process
- `warn` - Shows warnings but doesn't fail
