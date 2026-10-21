# Turn decisions into a small contract

Use a decision the user has made or the repository documents. State what it
protects and choose the narrowest verifier that establishes it. Do not freeze
every observed file, package, or current implementation automatically. Keep
subjective design and unspecified architectural intent in human review until
there is a meaningful observable criterion.

Name the evidence source, the property the verifier actually observes, and the
files or dependency categories it covers. Keep the contract's `name` and rule
IDs consistent with that coverage. In each `reason`, connect the project
decision to the narrower check and state material exclusions; do not leave
those limits only in the final report. For example, a rule inspecting direct
requirements in the root `go.mod` must not promise repository-wide absence of
external dependencies.

A release process reserving version changes for maintainers does not establish
a decision to freeze today's version; a scalar equality cannot verify who
authorized a change.

Inspect existing CI, tests, linters and build checks before adding a verifier.
Explain which property each check establishes, distinguishing checks already
configured or executed from recommendations. Keep useful existing checks; a
passing Linebind result does not replace them. Use a `command` rule when it
meaningfully connects an existing verifier to the contract, not simply to wrap
every CI step. Leave remaining gaps explicit instead of approximating syntax or
semantic checks with regexes.

## Contract shape

```yaml
version: 1
name: Frontend declared runtime packages
rules:
  - id: frontend.runtime-declarations
    kind: dependencies
    reason: >-
      New browser runtime package declarations require a separate project decision.
      Checks dependencies, optionalDependencies and peerDependencies in the root
      package.json; development declarations, other manifests and source imports
      are outside this rule.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies, optionalDependencies, peerDependencies]
      allow: [react, react-dom]
    remediation: Remove the unapproved dependency or propose a reviewed contract change.
```

This example applies only if that package decision is actually established.
Adapt it to the project; do not copy its allowlist as a universal frontend rule.
This example includes optional and peer declarations but permits development
tools. Choose those categories from the decision, not from which ones happen
to exist in today's manifest.

The root needs `version: 1`, a nonempty `name`, and 1–256 rules. Each rule needs
a stable unique `id`, `kind`, `reason`, and its matching configuration block.
`scope` contains repository-relative globs and is required except for
`require-files`. Use `/` separators and no absolute paths or `..` traversal.
Scope selects captured files; it does not require each listed path to exist.
With several manifests, deleting one can leave a document rule green. If those
inputs are mandatory, pair the value check with `require-files`; otherwise state
that file presence is outside the promise.
The parser rejects unknown fields, duplicate keys, YAML aliases, anchors, and
custom tags. `reason` and `remediation` explain a rule; neither is executable.

Optional common fields:

- `remediation`: an actionable next step when the rule fails.
- `required: false`: advisory instead of blocking; defaults to `true`.
- `compare: new`: block additional findings while preserving existing and
  resolved findings. Default `strict` checks all current violations. `new`
  requires complete analysis on both snapshots and is unavailable for a
  `command` with `format: exit` or a `co-change` rule. Choosing it is a debt policy, not a way to hide
  a task's new failure.

Before choosing `compare: new`, inspect existing findings. Legitimate comments,
strings or files outside the intended decision are not automatically technical
debt. Correct the verifier or scope instead of preserving false positives as
an accepted baseline.

## Available kinds

These are individual rule examples to put under `rules`, not mandatory
rules to install together.

### Related changes (v0.5.0)

```yaml
- id: migration-guide
  kind: co-change
  reason: Ship migration instructions with schema changes.
  scope: [migrations/**]
  co-change:
    require: [docs/migrations.md]
```

If any scoped file is added, removed or content-modified, each required pattern
must match an added or content-modified candidate file. A deletion does not
satisfy that obligation. No trigger means no obligation; omitted inputs on either
side remain unverified. Renames are deletion plus addition; mode-only changes
and LF/CRLF text checkout conversions do not count. This checks that an accompanying edit exists, not its quality.
Whitespace edits count. Do not claim it proves test coverage or documentation
accuracy, and do not add it without an explicit delivery decision.

### Required file presence

```yaml
- id: decisions-present
  kind: require-files
  reason: Contributors need the recorded architectural decisions.
  files:
    require: [docs/decisions.md]
```

Each `files.require` path or glob must match at least one captured file. An
optional `scope` narrows eligible files. Presence does not establish quality,
accuracy, or that an agent has read the document.

### Forbidden text

```yaml
- id: no-debug-marker
  kind: forbid-text
  reason: Temporary debug markers must be removed from shipped source.
  scope: [src/**/*.ts]
  text:
    pattern: DEBUG_ONLY
```

`text.pattern` uses Go/RE2 regular expressions on text, including comments and
strings. It does not parse imports or prove architectural boundaries. Use a
specialist verifier when syntax or module resolution matters.
For example, a regex for `console.log` can match a comment and miss
`console["log"]` or an alias. Keep a literal-marker rule literal; do not sell it
as a semantic ban on logging.

### Declared dependency names

Use the complete example above. `dependencies.format` accepts `npm` or `go`;
`python` and `cargo` require v0.4.0 or newer. Choose
exactly one of `allow: [names]` or `deny: [names]`. Names are exact, not globs.
An empty `allow: []` permits no direct dependencies; an empty deny list is
invalid. npm `sections` may contain `dependencies`, `devDependencies`,
`peerDependencies`, and `optionalDependencies`; omitting it checks all four.
For Go, use `scope: [go.mod]`, `format: go`, and no `sections`.

Python reads `pyproject.toml` sections `project.dependencies`,
`project.optional-dependencies` and `build-system.requires` by default. Names
use Python's lowercase/dot/underscore/hyphen normalization. Cargo reads
`dependencies`, `dev-dependencies` and `build-dependencies`, including target
tables and local `workspace.dependencies` declarations. `sections` can narrow
these defaults. Include all relevant manifests in `scope`. Dynamic Python
metadata and Cargo workspace inheritance remain unverified, as do non-registry
sources; neither format executes packaging tools or resolves a graph.

This checks declarations, not versions, vulnerabilities, lockfile integrity,
or the resolved transitive graph. Go checks direct `require` entries and skips
entries marked `// indirect`; local replacements can leave identity unverified.
A root-only scope does not cover separate nested modules. For Go projects,
`go mod tidy -diff` can detect a used dependency incorrectly marked indirect,
while build and test commands provide import and behavior evidence. These
checks can still pass with a correctly declared external dependency: they do
not enforce a project's dependency allowlist. Report their results separately
from Linebind's declaration check and retain any remaining coverage limits.

npm non-registry sources can also leave identity unverified. Do not promise
more than the manifest evidence.
For a decision such as no external runtime packages, cover `dependencies`,
`optionalDependencies` and `peerDependencies` in each relevant npm manifest.
Exclude a category only when the documented decision justifies it, not because
it is empty today. Probe each covered category; one alternate dependency test
does not establish coverage of the others. An undeclared import or a development
dependency imported by shipped code is outside this verifier's coverage. Use an
existing import/build check for that evidence, or report the narrower
declaration-only protection explicitly.

### JSON/YAML values

```yaml
- id: esm-package
  kind: document
  reason: Project tooling and imports rely on ECMAScript modules.
  scope: [package.json]
  document:
    selector: /type
    equals: module
```

Choose exactly one document condition: `required: true` requires an existing,
non-null value; `equals: VALUE` requires the selected scalar or null; or
`forbidden-values: [VALUES]` rejects those existing scalar values but allows
absence. Version v0.4.0 or newer also accepts `allowed-values: [VALUES]`
(a nonempty scalar list; missing values fail) or `pattern: REGEX` (an existing
string matching a Go/RE2 expression; anchor it for whole-string matching).
Confirm the installed verifier supports these conditions before using them.
This nested `document.required` differs from the rule's advisory flag.
Selectors use JSON Pointer-style segments, numeric array indexes, and `*` for
children. Files must be JSON, YAML, or YML. This is a focused assertion about
declarations, not full schema validation or proof about live systems or rows.

### External command

```yaml
- id: data-invariants
  kind: command
  reason: The fixed fixture must satisfy the transformation's invariants.
  scope: [data/fixture.csv, scripts/check_data.py]
  command:
    argv: [python3, scripts/check_data.py, data/fixture.csv]
    policy-files: [scripts/check_data.py]
    format: exit
    timeout: 30
    success-codes: [0]
    failure-codes: [1]
```

Use an existing meaningful verifier, or implement one only when the requested
scope justifies it. `argv` is an executable and arguments without implicit shell
expansion. Tools must be installed. Formats are `exit` (default),
`dependency-cruiser`, or `conftest`; the latter two require their actual supported
JSON output, not arbitrary output labeled as such. Timeout defaults to 30
seconds, maximum 300. Unexpected exit codes and incomplete output are
unverified, not evidence of a domain violation.

Declare every file controlling the verdict in `policy-files`, including local
helpers and imported policy files; those exact paths come from the base during
`check` and the candidate during `preview`. `scope` describes coverage; it does
not limit the files available to the external command. The disposable snapshot
contains no `.git` and does not provision ignored dependency directories. Read
[operations](operations.md#external-checks) before enabling `--allow-exec`.

## Test the decision, then report its limits

Run `validate`, then `preview` against a meaningful comparison snapshot. For
each rule, state the expected outcomes before making independent mutations:

- A permitted change must pass, including a legitimate case resembling a
  violation (such as a comment when the intended decision concerns code).
- A direct violation and a realistic indirect variant must fail: for example
  an optional dependency as well as a regular dependency, or an import through
  a re-export when an architecture verifier promises to follow that path.
- A missing or unreadable input needed for the verdict must not pass. Probe a
  newly added file or workspace to expose gaps in fixed paths and globs.
- A nearby property the verifier cannot establish must be explicitly listed as
  unprotected, even if its mutation leaves the result green.

Use a disposable Git fixture or copy; preserve the real repository's source and
approval state. Restore each mutation before the next probe and re-run the final
proposal. Retain each mutation's diff, command, expected outcome and actual
report. Distinguish executed probes from known but untested limitations; do not
combine an executed change and a hypothetical variant in one result row.
A missed promised violation or a blocked permitted
change calls for a better verifier, a narrower honest promise, or a human review
criterion. Do not relabel a failed probe to claim success. For `compare: new`,
exercise both accepted existing debt and an additional real violation.

Scopes matching no files are unverified except when required-file patterns
themselves establish absence. Symlinks, submodules, oversized inputs, missing
tools, or unavailable files can also prevent a complete check. A rule targeting
only a declaration cannot be advertised as a runtime or semantic guarantee.

For an evolution, report the changed decision, its rule diff, the old gate
result, the proposed preview result, and any protection removed. Keep the
original approved base fixed until the repository's review process adopts a
new one. Full format details and limits:
[configuration reference](https://github.com/mchl-schrdng/linebind/blob/main/docs/configuration.md).
