# Testers

Module-level **testers** execute a module's own test fixtures and compare the results against what the module declares it expects. They are the engine behind the [`dmt test`](../../internal/test/README.md) command.

## Testers vs. linters

`dmt` has two families of checks, and this package is the second one:

| | Linters (`pkg/linters`) | Testers (`pkg/testers`) |
|---|---|---|
| Question | "Is the module written correctly?" | "Does the module behave as it says it does?" |
| Input | The module's source files, analysed statically | The module's source **plus** its committed test fixtures |
| Applicability | Run on every module | Run only on modules that ship the fixtures a tester needs |
| CLI | `dmt lint` | `dmt test` |

A linter reads a manifest and flags a problem. A tester *runs* something — replays a conversion testcase, renders a chart — and checks the outcome against a committed expectation.

## The `Tester` interface

Every tester implements the interface in [`tester.go`](tester.go):

```go
type Tester interface {
	// Run executes the tester against the given module path.
	// Returns true if the tester was applicable to this module.
	Run(modulePath string) bool

	Name() string
	Desc() string
}
```

Two things are worth calling out:

- **`Run` returns applicability, not pass/fail.** It returns `true` when the module ships the inputs this tester needs (so the tester actually ran), and `false` when it does not (so the module is *skipped*). Pass/fail is reported out-of-band, by appending findings to the shared error list the tester is constructed with — never through the return value.
- **Findings go to a `*pkgerrors.TestErrorsList`.** Each concrete tester takes that list in its `New(...)` constructor and scopes it with `WithTestGroup(<id>)`. A finding at `pkg.Error` level makes `dmt test` exit non-zero.

## Available testers

| Tester (`Name()`) | Purpose | Applicable when the module has | Documentation |
|-------------------|---------|--------------------------------|---------------|
| `conversions` | Validates OpenAPI configuration conversions against the declared config version and replays their testcases | `openapi/conversions/` | [conversions/README.md](conversions/README.md) |
| `templates` | Renders the module's templates with per-case values and compares the output against committed golden snapshots | `templates-tests/` | [templates/README.md](templates/README.md) |

## How testers are run

Testers are not invoked directly. The [`internal/test`](../../internal/test/README.md) `Manager` registers all of them, discovers every module under the target path, runs each applicable tester per module, and prints a per-module result:

- `✅ [<tester>] <module>` — the tester ran and passed
- `❌ [<tester>] <module>` — the tester ran and reported failures (details follow)

Modules to which no tester is applicable are silently skipped.

```bash
# Validate conversions for all modules under the current directory
dmt test conversions

# Compare templates against golden snapshots for a single module
dmt test templates ./modules/my-module

# Refresh snapshots after intentional template changes
dmt test templates ./modules/my-module --update
```

Registration happens in `Manager.registerTesters` ([internal/test/manager.go](../../internal/test/manager.go)); the CLI subcommands and flags live in `cmd/dmt/root.go`. The `--update` flag reaches snapshot-based testers through the `WithUpdateSnapshots` option, and `WithTesters(<id>...)` restricts a run to specific testers (each `dmt test` subcommand uses it to run exactly one).

## Adding a tester

1. Create a subpackage `pkg/testers/<name>/` with a `Tester` type that implements the interface above, an `ID`/`name` constant (this is the string used by `Name()` and `WithTesters`), and a `New(errorList *pkgerrors.TestErrorsList, ...)` constructor that scopes the list with `WithTestGroup(ID)`.
2. In `Run`, detect applicability first: return `false` (skip) unless the module ships the inputs your tester needs; otherwise run the checks and append any failures to the error list, then return `true`.
3. Register the tester in `Manager.registerTesters` (append it to the `all` slice in [internal/test/manager.go](../../internal/test/manager.go)).
4. If it should be individually invokable, add a `dmt test <name>` subcommand in `cmd/dmt/root.go` (wire it with `test.WithTesters("<name>")`).
5. Add a `README.md` in the subpackage and a row to the table above.

## Package layout

```
pkg/testers/
├── README.md              # this file
├── tester.go              # the Tester interface
├── conversions/           # conversions tester (+ README, converter, tests)
└── templates/             # templates tester (+ README, testdata, tests)
```
