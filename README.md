# dmt

Deckhouse Module Tool - the swiss knife for your Deckhouse modules

## Ho to install

### go install (recommended method)
This is the simplest and fastest method to install the latest version. The command will download, compile, and install the binary in $GOPATH/bin (or ~/go/bin by default).
Command:
```shell
go install github.com/deckhouse/dmt@latest
```
After installation, add ~/go/bin to your PATH (if not already added):
In ~/.bashrc or ~/.zshrc: 
```shell
export PATH=$PATH:~/go/bin
source ~/.bashrc (or similar)
dmt --version
```

### download latest release
You can download any [release] (https://github.com/deckhouse/dmt/releases) that is compatible with your system.
At 
```shell
mkdir ~/Downloads/dmt && cd ~/Downloads/dmt
curl -L -o dmt-0.1.40-linux-amd64.tar.gz https://github.com/deckhouse/dmt/releases/download/v0.1.40/dmt-0.1.40-linux-amd64.tar.gz
tar -xzf dmt-0.1.40-linux-amd64.tar.gz
cd dmt-0.1.40-linux-amd64
./dmt --version
```
Add to path:
(A) Move the binary file to the system directory
```shell
sudo mv dmt /usr/local/bin/dmt
```

(B) Add the directory to ~/.bashrc or ~/.zshrc
1. Open the file: nano ~/.bashrc (or vim ~/.bashrc).
2. Add to the end:
```shell
export PATH="$PATH:$HOME/Downloads/dmt/dmt-0.1.40-linux-amd64"
```
(Replace the path with yours; use pwd for the exact one).
3. Save and apply: source ~/.bashrc.
4. Check: echo $PATH (should be your directory), then dmt --version.

For zsh: Replace ~/.bashrc with ~/.zshrc and use source ~/.zshrc.

## How to use

### Lint

You can run linter checks for a module:

```shell
dmt lint /some/path/<your-module>
```

or some pack of modules

```shell
dmt lint /some/path/
```

where `/some/path/` looks like this:

```shell
ls -l /some/path/
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 10 21:46 001-module-one
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 12 21:45 002-module-two
drwxrwxr-x 1 deckhouse deckhouse  4096 Nov 10 21:46 003-module-three
```

You can also run linter checks for multiple directories at once:

```shell
dmt lint /path/to/module1 /path/to/module2 /path/to/module3
```

Each directory is processed as a separate execution, and results are displayed for each directory individually.

### Bootstrap

Bootstrap a new Deckhouse module from template:

```shell
dmt bootstrap my-module-name
```

This command will:
- Download the official Deckhouse module template
- Extract it to the current directory (or specified directory)
- Replace template placeholders with your module name
- Configure CI/CD files based on your chosen platform

#### Options

- `--pipeline, -p`: Choose CI/CD platform (`github` or `gitlab`, default: `github`)
- `--directory, -d`: Specify target directory (default: current directory)
- `--repository-url, -r`: Use custom module template repository URL

#### Examples

Bootstrap a GitHub module:
```shell
dmt bootstrap my-awesome-module --pipeline github
```

Bootstrap a GitLab module in specific directory:
```shell
dmt bootstrap my-module --pipeline gitlab --directory ./modules/my-module
```

Use custom template repository:
```shell
dmt bootstrap my-module --repository-url https://github.com/myorg/custom-template/archive/main.zip
```

## Linters list

| Linter                                                   | Description                                                                  |
|----------------------------------------------------------|------------------------------------------------------------------------------|
| [container](pkg/linters/container/README.md)             | Check containers - duplicated names, env variables, ports, security context, liveness and readiness probes.|
| [hooks](pkg/linters/hooks/README.md)                     | Check hooks rules. |
| [images](pkg/linters/images/README.md)                   | Check images build instructions. |
| [module](pkg/linters/module/README.md)                   | Check module.yaml definition, openapi conversions, oss.yaml file.|
| [no-cyrillic](pkg/linters/no-cyrillic/README.md)         | Check cyrillic letters. |
| [openapi](pkg/linters/openapi/README.md)                 | Check openapi settings, crds. |
| [rbac](pkg/linters/rbac/README.md)                       | Check rbac rules. |
| [templates](pkg/linters/templates/README.md)             | Check templates rules, VPA, PDB settings, prometheus, grafana rules, kube-rbac-proxy, service target port. |

## Development Setup

### Pre-commit Hooks

To enable automatic linting before each commit, run:

```shell
make setup-hooks
```

This will install a pre-commit hook that:

- Runs fast lint checks before each commit
- Attempts to auto-fix issues when possible
- Prevents commits with linting errors

The hook uses `make lint-fast` for quick checks and `make lint-fix-fast` for auto-fixing.

### Available Make Targets

- `make setup-hooks` - Install pre-commit hooks
- `make lint` - Run full linting
- `make lint-fast` - Run fast linting (used by pre-commit hook)
- `make lint-fix` - Run full linting with auto-fix
- `make lint-fix-fast` - Run fast linting with auto-fix

## Configuration

You can exclude linters or setup them via the config file `.dmtlint.yaml`

### Global settings

```yaml
global:
  linters-settings:
    module:
      impact: warn | critical
    images:
      impact: warn | critical  
```
