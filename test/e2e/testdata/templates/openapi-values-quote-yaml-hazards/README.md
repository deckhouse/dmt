# YAML-injection e2e coverage for `openapi-values-quote`

Three e2e cases exercise the `openapi-values-quote` rule against the YAML-injection
threat model. The rule's guarantee is simple: an unconstrained OpenAPI string (no
`pattern`/`enum`/`format`) rendered **unquoted** is a hazard and must be flagged; the
same value is safe when quoted (`| quote`, wrapped, or a YAML-safe function) or when a
schema keyword restricts its character set.

| Case | Focus |
|------|-------|
| `openapi-values-quote-yaml-injection` | document-separator `\n---\n` (rogue-document injection), across scalar / array / map / nested / `with` / array-of-objects vectors |
| `openapi-values-quote-yaml-injection-safe` | negative control — the same dangerous values, all quoted/constrained → the rule must be silent and the render must emit no rogue document |
| `openapi-values-quote-yaml-hazards` | the rest of the family — everything below marked ✅ |

## Covered

Every payload below renders to **valid YAML** (so the module loads and the rule finding
can be asserted). The "render effect" column was verified against dmt's real render
(nelm + its YAML parser); the e2e case asserts the rule fires and the module loads.

| Class | Payload (unquoted) | Verified render effect |
|-------|--------------------|------------------------|
| Document separator | `foo\n---\n…` | injects a whole new document (a `Secret` appears in the object store) |
| Document end | `foo\n...\n` | `...` truncates the document — trailing fields silently dropped |
| Key injection | `foo\n  key: val` (indented to the field) | injects a sibling key into the current mapping (`injected: pwned`) — no new document needed |
| Bool coercion | `no` / `on` / `yes` | parsed as a boolean (`no` → `false`) — the "Norway problem" |
| Int coercion | `0755` / `1234567890` | parsed as an integer (`0755` → octal `493`) |
| Float coercion | `1.10` | parsed as a float (`1.1` — trailing zero lost) |
| Null coercion | `~` / `null` / empty | parsed as null |
| Comment truncation | `visible # secret` | everything after ` #` dropped as a comment |
| Flow sequence | `[a, b]` | string becomes a list |
| Flow mapping | `{a: b}` | string becomes a map |
| Tag indicator | `!badtag` | leading `!` read as a YAML tag; value rendered empty |
| Anchor indicator | `&a value` | leading `&` read as an anchor; token stripped |

## NOT covered — and why

### 1. Parse-breaking payloads abort module creation (cannot assert the rule)

Payloads that render to **invalid** YAML make dmt fail `NewModule` with a single
`manager: cannot create module` finding — the per-linter phase never runs, so the
`openapi-values-quote` finding cannot be observed in an e2e case that bakes the payload
into a value default. Verified fatal:

- bare `key: value` (colon-space) — `mapping value is not allowed in this context`
- leading `@` or `` ` `` — reserved indicator
- `*name` — reference to an undefined alias
- a column-0 `foo\nkey: val` (key injection whose indentation does **not** match)
- (same class: unbalanced flow `[a, b`, a leading tab that breaks indentation)

This is a limitation of the **synthetic-default e2e setup**, not of the rule: the rule
reads raw template source, so in a real module (real values, not a baked default) it
still flags these unquoted usages before they ever reach a cluster. The danger of these
payloads is instead the render abort itself.

### 2. The framework asserts on findings, not on rendered shapes

`expected.yaml` matches findings by `linter` / `rule` / `level` / `textContains`. It
cannot assert "the value became a bool", "a key was injected", or "the document was
truncated" — those render effects are verified by hand (see the probe approach in the
commit history) and documented in the case, but the machine assertion is limited to
"the rule flagged the unquoted usage" and "the module still rendered".

### 3. Out of the rule's scope (documented rule limitations)

The rule intentionally does **not** flag these, so they are not coverable as
"rule catches it" (see the rule's own limitations in
`pkg/linters/templates/README.md`):

- a value inside a block scalar (`|` / `>`) — already a literal string, must not be quoted
- a value passed through Helm `tpl` (template injection) or to an **external** template (helm_lib)
- `toYaml | nindent N` indentation attacks (whole-subtree injection)
- dynamic access (`dig` / `get` / `pluck`, `index` nested in another call)
- a map **key** emitted as `{{ $k }}:`
- a `{{ … }}` action spanning multiple physical lines

### 4. Parser-dependent coercions that do not fire here

Measured against dmt's actual parser: the YAML-1.1 boolean set (`yes/no/on/off`) **does**
coerce, but **timestamps** (`2020-01-01`) and **base-60 / sexagesimal** (`1:2:3`) do
**not** — they stay strings. So there is deliberately no coercion case for those; a case
asserting them would encode parser behavior the rule does not depend on.

### 5. DoS (billion-laughs / recursive anchors)

A recursive YAML bomb needs both an anchor definition and an alias that references it; a
single scalar value cannot form that structure in a way that survives rendering, and a
bare `*alias` errors fatally (see §1). Not meaningfully coverable at the value level.
