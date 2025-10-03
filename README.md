<div align="center">

# 🛠️ DMT - Deckhouse Module Tool

**The Swiss Knife for Deckhouse Module Development**

[![GitHub Release](https://img.shields.io/github/v/release/deckhouse/dmt)](https://github.com/deckhouse/dmt/releases)

[Features](#-features) •
[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Documentation](#-documentation) •
[Contributing](#-contributing)

</div>

---

## 📖 Overview

**DMT** (Deckhouse Module Tool) is a comprehensive command-line tool designed to streamline the development, testing, and maintenance of [Deckhouse Kubernetes Platform](https://deckhouse.io/) modules. It provides powerful linting capabilities, project bootstrapping, and validation tools to ensure your modules meet quality standards.

### Why DMT?

- ✅ **Quality Assurance**: Comprehensive linting with 9 specialized linters
- 🚀 **Fast Development**: Bootstrap new modules in seconds
- 🔧 **Configurable**: Fine-tune linting rules per project
- 🎯 **CI/CD Ready**: Perfect for automated pipelines

---

## 🎯 Features

### 🔍 Advanced Module Linting

DMT includes **9 specialized linters** to validate different aspects of your Deckhouse modules:

| Linter | Purpose | Key Checks |
|--------|---------|------------|
| [**Container**](pkg/linters/container/README.md) | Container configuration validation | Duplicate names, env vars, security contexts, probes, resource limits |
| [**Documentation**](pkg/linters/docs/README.md) | Documentation quality | README presence, bilingual support, no cyrillic in English docs |
| [**Hooks**](pkg/linters/hooks/README.md) | Hook validation | Hook syntax, ingress configurations |
| [**Images**](pkg/linters/images/README.md) | Image build instructions | Dockerfile best practices, werf configuration |
| [**Module**](pkg/linters/module/README.md) | Module structure | module.yaml format, OpenAPI conversions, oss.yaml, license files |
| [**NoCyrillic**](pkg/linters/no-cyrillic/README.md) | Character encoding | Cyrillic characters in code/config files |
| [**OpenAPI**](pkg/linters/openapi/README.md) | OpenAPI schemas | Schema validation, CRD definitions, naming conventions |
| [**RBAC**](pkg/linters/rbac/README.md) | Security policies | Role bindings, service accounts, wildcards |
| [**Templates**](pkg/linters/templates/README.md) | Kubernetes templates | VPA/PDB settings, Prometheus rules, Grafana dashboards, service ports |

### 🚀 Module Bootstrapping

Quickly scaffold new Deckhouse modules with best practices:

- **GitHub/GitLab** CI/CD integration
- Pre-configured project structure
- Template customization
- Automated setup for common patterns

### ⚙️ Flexible Configuration

- **Per-module config**: Override rules for specific needs
- **Impact levels**: Control severity (error/warn)
- **Exclusion rules**: Skip checks for specific resources

---

## 📦 Installation

### Method 1: Install Script (Recommended)

Quick one-line installation for Linux and macOS:

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/dmt/main/tools/install.sh)"
```

<details>
<summary>Alternative installation commands</summary>

**Using wget:**
```bash
sh -c "$(wget -qO- https://raw.githubusercontent.com/deckhouse/dmt/main/tools/install.sh)"
```

**Install specific version:**
```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/dmt/main/tools/install.sh)" "" --version v1.0.0
```

**Install to custom directory:**
```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/dmt/main/tools/install.sh)" "" --install-dir ~/bin
```

See [installation guide](tools/README.md) for more options.
</details>

### Method 2: Download Binary

Download the latest release for your platform from the [releases page](https://github.com/deckhouse/dmt/releases).

**Supported Platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Method 3: Go Install

If you have Go installed (requires Go 1.24+):

```bash
go install github.com/deckhouse/dmt@latest
```

> **Note**: Ensure `~/go/bin` is in your PATH after installation.

### Method 4: Using trdl

[trdl](https://github.com/werf/trdl) is a tool release delivery system that provides automatic updates and channel management:

```bash
# Install from stable channel
trdl use dmt stable

# Or install from alpha channel for latest features
trdl use dmt alpha
```

**Benefits:**
- Automatic updates
- Channel-based releases (stable, alpha)
- Cross-platform support
- Version management

> **Note**: Requires [trdl](https://github.com/werf/trdl) to be installed first.

### Verify Installation

```bash
dmt --version
```

---

## 🚀 Quick Start

### Lint Your Modules

**Lint a single module:**
```bash
dmt lint /path/to/your-module
```

**Lint multiple modules:**
```bash
dmt lint /path/to/modules/
```

**Lint specific directories:**
```bash
dmt lint ./module-1 ./module-2 ./module-3
```

**Example directory structure:**
```
/path/to/modules/
├── 001-module-one/
├── 002-module-two/
└── 003-module-three/
```

### Bootstrap a New Module

**Create a new module with GitHub CI/CD:**
```bash
dmt bootstrap my-awesome-module
```

**Create with GitLab CI/CD:**
```bash
dmt bootstrap my-module --pipeline gitlab
```

**Specify target directory:**
```bash
dmt bootstrap my-module --directory ./modules/my-module
```

**Use custom template:**
```bash
dmt bootstrap my-module --repository-url https://github.com/myorg/template/archive/main.zip
```

> **Note**: Module names must be in kebab-case (e.g., `my-module-name`)

---

## 📚 Documentation

### Configuration

DMT can be configured using a `.dmtlint.yaml` file in your project root.

#### Configuration

```yaml
linters-settings:
  container:
    impact: error
    exclude-rules:
      # Exclude specific resources from checks
      security-context:
        - kind: Deployment
          name: legacy-app
          container: main
      
      resources:
        - kind: DaemonSet
          name: node-exporter
  
  images:
    impact: warn
    exclude-rules:
      skip-image-file-path-prefix:
        - images/special-case/
  
  documentation:
    impact: error
  
  templates:
    impact: error
    exclude-rules:
      vpa-absent:
        - kind: Deployment
          name: one-off-job
```

### Command Line Options

#### Lint Command

```bash
dmt lint [directories...] [flags]
```

**Flags:**
- `--values-file, -v`: Specify custom values file
- `--linter`: Run specific linter only
- `--hide-warnings`: Hide warning-level issues
- `--abs-path`: Show absolute paths in output
- `--show-ignored`: Display ignored issues
- `--log-level`: Set logging verbosity (debug, info, warn, error)

**Examples:**
```bash
# Run only container linter
dmt lint ./my-module --linter container

# Hide warnings
dmt lint ./my-module --hide-warnings

# Use custom values
dmt lint ./my-module --values-file custom-values.yaml

# Debug mode
dmt lint ./my-module --log-level debug
```

#### Bootstrap Command

```bash
dmt bootstrap <module-name> [flags]
```

**Flags:**
- `--pipeline, -p`: CI/CD platform (`github` or `gitlab`, default: `github`)
- `--directory, -d`: Target directory (default: current directory)
- `--repository-url, -r`: Custom template repository URL

**Examples:**
```bash
# GitHub project
dmt bootstrap awesome-module --pipeline github

# GitLab project in specific directory
dmt bootstrap my-module -p gitlab -d ./modules/my-module

# Custom template
dmt bootstrap my-module -r https://example.com/template.zip
```

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. **Report Bugs**: Open an issue describing the problem
2. **Suggest Features**: Share your ideas for improvements
3. **Submit PRs**: Fix bugs or add features
4. **Improve Docs**: Help make documentation better

### Development Guidelines

- Follow Go best practices and idioms
- Add tests for new features
- Update documentation
- Run `make lint` before committing
- Use conventional commit messages

---

## 🔗 Links

- **Website**: [deckhouse.io](https://deckhouse.io/)
- **Issues**: [Report a bug or request a feature](https://github.com/deckhouse/dmt/issues)
- **Releases**: [Download binaries](https://github.com/deckhouse/dmt/releases)
- **Documentation**: [Deckhouse Documentation](https://deckhouse.io/documentation/)

---

## 🌟 Support

If you find DMT helpful, please consider:
- ⭐ Starring the repository
- 🐛 Reporting bugs
- 💡 Suggesting features
- 📖 Contributing to documentation
- 🔀 Submitting pull requests
