# Linters and rules

Every lint check in dmt is a **rule**; rules are grouped into **linters**. Both
have a single interface, so a linter's rule set is data rather than a sequence
of hand-written calls.

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

// internal/manager/manager.go
type Linter interface {
	Name() string
	Lint(ctx context.Context)
}
```

A rule receives everything it needs — the target it inspects and an already
scoped error list — through its constructor. `Check` therefore takes nothing but
a context, no matter what the rule actually looks at.

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
	if l.module == nil || l.module.GetPath() == "" {
		return
	}

	for _, rule := range l.rules() {
		rule.Check(ctx)
	}
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

A third source of run-to-run noise, unrelated to any rule: the `markdownlint`
docs rule iterates `results` — a map — so its findings come out in arbitrary
order. Sorting the output normalizes it.
