# Linters and rules

Every lint check in dmt is a **rule**; rules are grouped into **linters**, and a
**scope** decides which linters run and which of their rules each one is asked
for. Both rules and linters have a single interface, so a linter's rule set is
data rather than a sequence of hand-written calls — which is what lets a scope
select from it.

This mirrors `internal/verify` in
[d8-package-plugin](https://fox.flant.com/deckhouse/runtime/plugins/d8-package-plugin/-/tree/main/internal/verify),
which is where this shape comes from.

## The interfaces

```go
// pkg/rule.go
type Rule interface {
	GetName() string
	Check(ctx context.Context)
}

// pkg/scopes/scope.go
type Linter interface {
	GetName() string
	Lint(ctx context.Context)
}
```

A rule receives everything it needs — the target it inspects and an already
scoped error list — through its constructor. `Check` therefore takes nothing but
a context, no matter what the rule actually looks at.

## Scopes

A scope is a source a module is read from. There are three:

| Scope | Source | Run by |
|---|---|---|
| `static` | the committed source tree | `dmt lint <dir>` |
| `bundle` | the packaged module image, `<repo>:<tag>` | `dmt lint --remote <repo>:<tag>` |
| `release` | the release metadata image, `<repo>/release:<tag>` | the same command |

`--remote` runs both image scopes off one reference: it pulls each image,
unpacks it to a temporary directory and lints that as a module. The module
behind an image comes from `modules.NewRemoteModule`, which skips the chart load
and the render — so `GetChart`, `GetObjectStore` and `GetValues` are nil there,
and the `release` and `bundle` tables must not ask for a rule that reads them.

**Linters and rules know nothing about scopes.** A linter is handed its config
and a `set.Set` of rule IDs, and that is the whole of what a scope tells it:

```go
func (h *Hooks) Lint(ctx context.Context) {
	pkg.RunRules(ctx, h.ruleIDs, h.rules())
}
```

`pkg.RunRules` checks only the rules whose IDs were asked for. **An empty set
means no rule at all, never every rule** — silently running everything when a
table entry is missing looks like a clean module rather than a missing check.

A linter that does observable work *outside* its rules must return early when it
is asked for nothing. Two do: `container.Lint` calls `CollectObjectContainers`,
which reports a finding of its own, and `templates.rules()` reports one when the
`monitoring/` read fails. Both check `l.ruleIDs.Size() == 0` first. This is the
reason the linter runs its own rules instead of the scope running them for it —
the scope cannot know which linters have work like this, and could not suppress
it if it did.

This is the one place dmt deliberately diverges from the reference
implementation. There each linter exports its own `var Scopes` and gates every
rule with `if l.runs(ruleID)`, so the linter has to know the scopes it lives in.
Here `pkg/scopes` owns both tables and the linters stay unaware.

Membership is declared in code, not in config: `.dmtlint.yaml` tunes severities
and can silence a rule with an ignored impact, but it cannot switch one on or
off. Note the difference between the two — a rule a scope never asks for
produces nothing at all, while `impact: ignored` silences a rule that *did* run
and still counts toward the ignored tally.

### Where a scope's severities come from

Membership is code, severity is config, and each scope reads its own section of
`.dmtlint.yaml`:

| Scope | Section |
|---|---|
| `static` | `global.linters-settings` |
| `bundle` | `remote.bundle` |
| `release` | `remote.release` |

`Scope.Settings` is the only place that mapping lives, and `remotelint` hands the
branch it returns to `modules.NewRemoteModule` rather than the whole root config —
a remote scope has no way to reach the source tree's settings even by accident.
The sections do not inherit from one another: an image is linted with the
severities written for it, or with the built-in defaults.

### The table is the authority

One scope is one file in `pkg/scopes`, and each holds a table — `staticRules`,
`releaseRules`, `bundleRules` — of the rule IDs that scope asks each linter for.
That table is the only statement of membership. A linter does not publish the
list of rules it carries, and **nothing checks a scope's table against that
list** — deliberately.

The tempting check is "a scope must ask every linter for all of its rules". It
held while `static` was alone and stopped holding the moment `release` and
`bundle` landed: `release-layout` belongs to a built image and never runs over a
source tree, `markdownlint` is the other way round. A check like that would push
back against the very thing scopes exist to express, so the tables are written
out by hand and trusted.

What that buys, and what it costs:

- a rule can be in one scope and not another with no ceremony — add its ID where
  it belongs and nowhere else;
- a rule added to a linter's `rules()` and forgotten in every table **silently
  does not run**. `pkg/scopes/scopes_test.go` cannot catch that; it only checks,
  for every scope, that the table's keys match the linters that scope builds, and
  that none of them is asked for an empty set.

Tests that need a linter exercised whole derive the ID set from `rules()` rather
than naming one — see `everyRule` in `pkg/linters/container/container_test.go`.
Written-out subsets in tests go stale the same way, just more quietly.

### Adding a rule

Write the rule, add it to the linter's `rules()`, then add its ID to every scope
table in `pkg/scopes` that should run it. Nothing will remind you.

## Writing a rule

```go
const ReadmeRuleName = "readme"

type ReadmeRule struct {
	pkg.RuleMeta
	pkg.PathRule // optional: exclusion matcher, if the rule is configurable

	module    pkg.Module
	errorList *errors.LintRuleErrorsList
}

func NewReadmeRule(m pkg.Module, errorList *errors.LintRuleErrorsList) *ReadmeRule {
	return &ReadmeRule{
		RuleMeta:  pkg.RuleMeta{Name: ReadmeRuleName},
		module:    m,
		errorList: errorList.WithRule(ReadmeRuleName),
	}
}

var _ pkg.Rule = (*ReadmeRule)(nil)

func (r *ReadmeRule) Check(_ context.Context) {
	// ...
	r.errorList.WithFilePath(path).Error("README.md file is missing in docs/ directory")
}
```

Conventions:

- **Constructor parameter order**: `New<X>Rule(<config/exclusions>, <target>, errorList)`.
  The error list is always last.
- **`WithRule` belongs in the constructor**, not in `Check`. Scopes that vary
  per finding (`WithFilePath`, `WithObjectID`, `WithValue`, `WithLineNumber`)
  stay at the emit site.
- **Take `pkg.Module`, not `*modules.Module`.** The interface covers everything
  a rule needs and makes the rule mockable (`internal/mocks/module_mock.go`,
  regenerate with `make generate-mocks`).
- **A rule that inspects many objects loops internally.** The linter does not
  iterate for it — that is what keeps `Check(ctx)` uniform:

  ```go
  func (r *ServicePortRule) Check(_ context.Context) {
      for _, object := range r.module.GetStorage() {
          r.checkObject(object)
      }
  }
  ```

- **Add `var _ pkg.Rule = (*XRule)(nil)`.** Forgetting `Check` then fails to compile.
- **Keep constructors free of I/O.** Linters build their whole rule set before
  running any of it.
- **One rule, one precondition.** If a check needs different gating from its
  neighbour, it is a separate rule — see `PrometheusRule` (module `monitoring/`
  files) versus `PromtoolRule` (rendered objects), which share a rule ID but
  not a precondition.

## Writing a linter

```go
func (l *Documentation) Lint(ctx context.Context) {
	if l.module.GetPath() == "" {
		return
	}

	pkg.RunRules(ctx, l.ruleIDs, l.rules())
}

func (l *Documentation) rules() []pkg.Rule {
	m := l.module
	errorList := l.ErrorList.WithModule(m.GetName())

	return []pkg.Rule{
		rules.NewReadmeRule(m, errorList.WithMaxLevel(l.cfg.Rules.ReadmeRule.GetLevel())),
		// ...
	}
}
```

`WithLinterID` and the linter-level `WithMaxLevel` are applied in the linter's
`New`; the rule-level `WithMaxLevel` is applied where the rule set is built.

## Migration status

All nine linters implement `Lint(ctx)`; every rule implements `pkg.Rule`. The
`legacyAdapter`/`legacyLinter` shim that carried unmigrated linters is gone.

The linter list no longer lives in `internal/manager`: `getLintersForModule` and
the `Linter` interface moved to `pkg/scopes`, and `manager.NewManager` takes the
scope to run. One scope is one file in `pkg/scopes`, selected by `Scope.Linters`;
an unknown scope logs an error and yields no linters.

Only `static` exists, and it is what `dmt lint` has always done, so the change is
output-neutral — verified by diffing every `test/e2e/testdata/*/*/module` fixture
before and after (see below on how).
There is no `--scope` flag yet: there is nothing to choose between, and
`internal/flags` are process globals that `test/e2e` already has to serialise
with `initLintFlagsOnce`.

Linters take `pkg.Module`, not `*modules.Module`, so a linter can be driven end
to end from a unit test with `mocks.NewModuleMock`.

### Rules that need pre-computed input

`container` is the one linter whose rules do not each derive their own input.
Its old dispatcher extracted containers per object behind two gates — an object
whose containers fail to parse, or which has none, drops out — and the
extraction reports a finding of its own. Had each of the 13 container rules
called `GetAllContainers` itself, that one finding would have become 13.

So `Lint` calls `rules.CollectObjectContainers` once and hands the result to the
rules, the way the reference implementation parses `oss.yaml` once and passes
the components down. Reach for this only when extraction is shared *and*
observable; when a rule can find its own input, it should.

When a linter iterates objects or files for its rules, move that loop into the
rule and keep the per-item body in its own method. Inlining it into the loop
turns the body's early returns into early returns from `Check`, which silently
drops every later item.

Keep that per-item method **unexported and callable on its own**, and check what
the existing unit tests feed it before pointing them at `Check`. `no-cyrillic`
is the cautionary case: its table test asserts that `guide.ru.md`, `README_RU.md`
and `index_ru.html` are skipped *by the rule's regexes*, but the walk in `Check`
only yields `.yaml/.yml/.json/.go`. Rewriting those tests to call `Check` would
leave them passing while silently testing the extension filter instead — so they
still call `checkFile` directly.

The test rule of thumb: point unit tests at `Check` only when **nothing sits
between the entry point and the body** — no walk, no filter. `module` qualifies
(every rule's only input is the module root, so a mock with the right
`GetPath()` is an exact substitute) and its tests drive `Check`. `no-cyrillic`
and `openapi` do not, and theirs keep calling the per-item method.

One more trap when the error list moves into the constructor: tests that reset
`errorList` mid-test and reuse the rule. The rule then still holds the *old*
list, so later assertions read an empty one and pass vacuously. Re-create the
rule after every reset — `module_yaml_test.go` needed this in 11 places. Worth
mutation-testing the result: break the rule and confirm the tests fail.

### Pre-existing anomalies, preserved on purpose

Migrations here are output-neutral, verified by diffing `dmt lint` over every
`test/e2e/testdata/*/*/module` fixture before and after. Where a linter already
behaved oddly, the oddity was carried over rather than fixed, so that the diff
stays empty and the fix can be reviewed on its own. Known ones in `rbac`:

- `user-authz` never stamps a rule ID — its findings carry an empty `RuleID`
  even though `UserAuthZRuleName` exists (`rules/user-authz.go`).
- `pkg.RBACLinterConfig.Rules` is populated from config but never applied. Note
  it is also *degenerate*: `mapSimpleLinterRules` (`internal/modules/module.go`)
  fills each of these `RuleConfig`s from the linter-level impact
  (`SetLevel("", <linter impact>)`), which the linter already applies in `New`.
  Since `WithMaxLevel` only ratchets down, wiring them up would be a provable
  no-op. The same holds for `hooks`, `openapi` and `no-cyrillic`: none of them
  has a per-rule `impact` path in `pkg/config/global/global.go`. The real task
  is either to add one or to delete these `RuleConfig`s as dead weight — not to
  "hook up" what is already applied.
- `PlacementRule` skips `templates/rbac-for-us.yaml` and
  `templates/rbac-to-us.yaml` entirely: the RBAC-v2 guard
  `strings.HasPrefix(shortPath, "templates/rbac")` also matches those two
  paths, so only nested `templates/<component>/rbac-*.yaml` is ever checked.

And in `images`:

- The `dockerfile` and `distroless` rules are **dead**. Both guard their walk
  with `fsutils.IsFile(<module>/images)`, but `images` is a directory and
  `IsFile` is `err == nil && !fi.IsDir()`, so both return on the first line and
  no Dockerfile is ever inspected. The fix is `IsDir`, but it surfaces findings
  across every real module, so it needs its own PR (and e2e fixtures — testdata
  currently contains no `Dockerfile` at all).
- `SkipImageFilePathPrefix` is mapped from config but read by nobody:
  `ImageRule` uses `SkipDistrolessFilePathPrefix` instead. The
  `skip-image-file-path-prefix` key therefore does nothing.
- The linter used to stamp rule IDs at the call site (`WithRule("image")` and
  friends). Every rule already stamped its own, later and therefore winning — so
  `"image"` never reached the output, where the rule reports as `dockerfile`.
  The call-site stamping is gone; `WithRule` now lives in each constructor.

And in `templates`:

- `PDBRule` reports only **one** offending controller per module. Its loop over
  `GetStorage()` ends with `return` — not `continue` — once a controller with no
  PodDisruptionBudget is found (`rules/pdb.go`). Since `GetStorage()` is a Go
  map, which controller gets named varies between runs: on the
  `templates/vpa-misconfigured` fixture, 10 runs of the same binary named
  `auto-app` 9 times and `initial-app` once. Predates the migrations
  (`git show 1fed9a8~1:pkg/linters/templates/rules/pdb.go`).

  This is why the before/after diffs are taken more than once: a single flaky
  line is the linter's own nondeterminism, not a regression. Confirm by diffing
  the baseline binary against *itself*.

- `PrometheusRule` and `PromtoolRule` are two distinct rules sharing the ID
  `prometheus-rules` (`rules/prometheus_rules.go:45` and `:199`). Since a scope
  selects by ID, it can only ask for both or neither. Splitting the IDs would
  change the `RuleID` of existing findings, so it needs its own PR.

A third source of run-to-run noise, unrelated to any rule: the `markdownlint`
docs rule iterates `results` — a map — so its findings come out in arbitrary
order. Sorting the output normalizes it.
