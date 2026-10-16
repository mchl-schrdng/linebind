# Turn decisions into a small contract

Use a decision the user has made or the repository documents. State what it
protects and choose the narrowest verifier that establishes it. Do not freeze
every observed file, package, or current implementation automatically. Keep
subjective design and unspecified architectural intent in human review until
there is a meaningful observable criterion.

## Map the decision before writing the rule

For an architecture boundary, inspect entry points, source directories, imports
and existing tests. Distinguish the observed implementation from the intended
design: an existing dependency is not automatically an allowed dependency.
Record the small set of module responsibilities, permitted dependencies and
exceptions that the project has actually chosen. Include what happens when a
new directory or source format appears; silently unclassified code leaves a
hole in the map.

Keep one executable source of truth. If dependency-cruiser already holds the
layer policy, reference that file through `command.policy-files`; do not create
a second Linebind graph to keep synchronized. A diagram can explain the policy
but does not enforce it. Module edges also do not establish runtime call paths,
business behavior or whether the contents of a correctly placed module belong
there. See the [architecture example](https://github.com/mchl-schrdng/linebind/tree/main/examples/dependency-cruiser)
for a bounded, tested policy.

## Contract shape

```yaml
version: 1
name: Frontend decisions
rules:
  - id: frontend.runtime-packages
    kind: dependencies
    reason: New browser runtime packages require a separate project decision.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom]
    remediation: Remove the unapproved dependency or propose a reviewed contract change.
```

This example applies only if that package decision is actually established.
Adapt it to the project; do not copy its allowlist as a universal frontend rule.

The root needs `version: 1`, a nonempty `name`, and 1–256 rules. Each rule needs
a stable unique `id`, `kind`, `reason`, and its matching configuration block.
`scope` contains repository-relative globs and is required except for
`require-files`. Use `/` separators and no absolute paths or `..` traversal.
The parser rejects unknown fields, duplicate keys, YAML aliases, anchors, and
custom tags. `reason` and `remediation` explain a rule; neither is executable.

Optional common fields:

- `remediation`: an actionable next step when the rule fails.
- `required: false`: advisory instead of blocking; defaults to `true`.
- `compare: new`: block additional findings while preserving existing and
  resolved findings. Default `strict` checks all current violations. `new`
  requires complete analysis on both snapshots and is unavailable for a
  `command` with `format: exit`. Choosing it is a debt policy, not a way to hide
  a task's new failure.

## Five available kinds

These are individual rule examples to put under `rules`, not five mandatory
rules to install together.

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

### Declared dependency names

Use the complete example above. `dependencies.format` is `npm` or `go`; choose
exactly one of `allow: [names]` or `deny: [names]`. Names are exact, not globs.
An empty `allow: []` permits no direct dependencies; an empty deny list is
invalid. npm `sections` may contain `dependencies`, `devDependencies`,
`peerDependencies`, and `optionalDependencies`; omitting it checks all four.
For Go, use `scope: [go.mod]`, `format: go`, and no `sections`.

This checks declarations, not versions, vulnerabilities, lockfile integrity,
or the resolved transitive graph. Go checks direct `require` entries; local
replacements can leave identity unverified. npm non-registry sources can also
leave identity unverified. Do not promise more than the manifest evidence.

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
absence. This nested `document.required` differs from the rule's advisory flag.
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

Run `validate`, then `preview` against a meaningful comparison snapshot. Before
testing, state the expected outcome of a few independent changes:

- A permitted refactor or new module within the chosen design must pass.
- A direct violation must fail, as must a realistic indirect form of the same
  violation: for example a re-export through a shared module.
- An unanalysable input in the declared scope must not produce a successful
  verification. Also probe a new path or file format that could escape that scope.
- A nearby intention outside the verifier's coverage must stay explicitly
  unprotected; do not present a passing result as evidence for it.

Use a disposable Git fixture or copy, keep source edits and the approval state
of the real repository intact, then remove the temporary repository. Retain the
small useful regression cases and their observed outcomes, not generated repo
trees. A rule that misses its promised violation needs a stronger verifier or
a narrower stated promise; do not change the expectation just to make the test
pass. For `compare: new`,
include existing debt and a new finding so the comparison itself is exercised.

Scopes matching no files are unverified except when required-file patterns
themselves establish absence. Symlinks, submodules, oversized inputs, missing
tools, or unavailable files can also prevent a complete check. A rule targeting
only a declaration cannot be advertised as a runtime or semantic guarantee.

For an evolution, report the changed decision, its rule diff, the old gate
result, the proposed preview result, and any protection removed. Keep the
original approved base fixed until the repository's review process adopts a
new one. Full format details and limits:
[configuration reference](https://github.com/mchl-schrdng/linebind/blob/main/docs/configuration.md).
