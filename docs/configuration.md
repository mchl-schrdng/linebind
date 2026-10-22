# The repository contract

Linebind reads a versioned YAML contract. The default path is `.linebind.yml`;
`--contract` selects another repository-relative path. `validate` checks the
local file's syntax and schema. `check` evaluates the contract stored in the
chosen base revision. `preview` evaluates the candidate's proposed contract
against the same comparison base; policy edits alone do not fail it.

```yaml
version: 1
name: Project decisions
rules:
  - id: architecture-decisions-present
    kind: require-files
    reason: Reviewers need the decisions that explain the module boundaries.
    remediation: Restore docs/architecture.md or propose a contract change.
    files:
      require: [docs/architecture.md]
```

The contract contains one YAML mapping, a nonempty `name`, and 1–256 rules.
Unknown fields, duplicate keys, anchors, aliases and custom tags are rejected.
Use literal `true` and `false` for booleans.

## Common rule fields

| Field | Meaning |
| --- | --- |
| `id` | Stable, unique identifier: 1–128 letters, digits, dots, underscores or hyphens; begins with a letter or digit. |
| `kind` | One of `require-files`, `forbid-text`, `dependencies`, `document`, `co-change`, `command`. |
| `reason` | Required explanation of the project decision. |
| `scope` | Repository-relative globs selecting files. Required except for `require-files`. |
| `required` | Defaults to `true`. Set `false` for an advisory check. |
| `compare` | `strict` by default; `new` compares findings with the base. |
| `remediation` | Optional next step shown with findings. |

Use exactly one configuration block matching `kind`: `files`, `text`,
`dependencies`, `document`, `co-change` or `command`.

Paths use `/` on every platform. Globs support `**`, for example `src/**/*.ts`.
They must be relative to the repository root: no absolute paths or `..`
traversal. A candidate scope matching no files is unverified, except for
`require-files` (missing files are violations) and `co-change` (no trigger means
no obligation). For `compare: new`, v0.5.0 also permits a known empty base scope
for native text, dependency and document checks; see [comparison](#compare-existing-debt).

## Require files

Each pattern in `files.require` must match at least one captured file. When
`scope` is omitted, the required patterns also define the scope. When provided,
`scope` narrows the files eligible to satisfy those patterns.

```yaml
- id: migration-notes
  kind: require-files
  reason: Schema changes need a documented migration procedure.
  files:
    require: [docs/migrations.md]
```

This establishes presence, not the document's accuracy or completeness.

## Co-change

Requires v0.5.0 or newer. When any file in `scope` is added, removed or changes
contents between the base and candidate, **each** pattern in `co-change.require`
must match at least one added or content-modified file that exists in the candidate.

```yaml
- id: migration-guide
  kind: co-change
  reason: Ship migration instructions with schema changes.
  scope: [migrations/**]
  co-change:
    require: [docs/migrations.md]
  remediation: Describe how to apply or revert this migration.
```

Adding a migration without updating the guide crosses the check. Adding the
guide or changing its contents satisfies it. Deleting the guide does not.
If no scoped file changed, the check passes even when no trigger file exists.
Use `require-files` separately when the companion must always be present.

The comparison uses the two captured endpoints, not intermediate commits.
Renames count as removal plus addition. Mode-only changes and Git's LF/CRLF
text checkout conversions do not count.
Every required pattern is independent; one file can satisfy multiple patterns.
Keep trigger and companion patterns separate: a changed file matching both
can satisfy its own obligation.
There is no filename pairing, AST analysis or semantic comparison. Any content
edit, including whitespace, counts; this checks delivery of an accompanying
edit, not its quality or the sufficiency of tests. Keep normal review and tests.

Omitted inputs under either the trigger or companion patterns leave the check
unverified. `compare: new` is rejected because this rule already compares the
base and candidate. `required: false` makes the obligation advisory.
`check` takes this rule from the trusted base; changing it in a candidate cannot
authorize that candidate. `preview` evaluates the proposed rule instead.

## Forbid text

`text.pattern` is a Go/RE2 regular expression applied to UTF-8 file contents.

```yaml
- id: no-debug-marker
  kind: forbid-text
  reason: The temporary debug marker must not reach committed source.
  scope: [src/**/*.py]
  text:
    pattern: 'DEBUG_ONLY'
```

This is a lexical check: comments and string literals are included. It does not
parse the language, resolve aliases, follow imports or establish semantic
equivalence. Use a specialist tool for those questions. Invalid UTF-8 or
unavailable input is reported as incomplete verification.

## Dependency names

`python` and `cargo` require v0.4.0 or newer. The v0.3.0
release supports `npm` and `go` only; older verifiers reject unsupported formats.

Choose a manifest format and exactly one name policy: `allow` or `deny`.
Names are exact, not globs. An empty `allow: []` permits no direct dependencies;
an empty deny list is invalid.

```yaml
- id: runtime-packages
  kind: dependencies
  reason: New runtime packages need an explicit project decision.
  scope: [package.json]
  dependencies:
    format: npm
    sections: [dependencies, optionalDependencies]
    allow: [react, react-dom]
```

For `npm`, the supported sections are `dependencies`, `devDependencies`,
`optionalDependencies` and `peerDependencies`. If `sections` is omitted, all
four are inspected. npm aliases are checked using both the declared name and
the aliased package identity. Non-registry sources produce a diagnostic because
the underlying package identity is not established.

For `go`, use `scope: [go.mod]` and `format: go`, with no `sections`. Linebind
checks `require` entries not marked `// indirect`; versioned replacements are
also checked by name. Local replacements leave package identity unverified.

For `python`, use `scope: [pyproject.toml]` and `format: python`. By default,
Linebind checks `project.dependencies`, all `project.optional-dependencies`
groups, and `build-system.requires`; `sections` can select any of these three.
Names are normalized to lowercase with runs of `.`, `_` and `-` replaced by
`-`, including names in `allow` and `deny`. All declared extras and environment
branches are inspected; markers are never evaluated against the host. Version
specifier syntax is read but ranges are not resolved or validated for
satisfiability. This is static [PEP 621 metadata](https://packaging.python.org/en/latest/specifications/pyproject-toml/),
not pip, Poetry or uv resolution. Dependency groups, lockfiles and tool-specific
development dependencies are outside its scope. Selected dynamic metadata,
missing project metadata, direct URL references, Poetry dependency declarations
and uv source overrides leave verification incomplete.

```yaml
- id: python-packages
  kind: dependencies
  reason: Runtime and optional packages need an explicit decision.
  scope: [pyproject.toml]
  dependencies:
    format: python
    sections: [project.dependencies, project.optional-dependencies]
    allow: [requests, pydantic]
```

For `cargo`, use `scope: ['**/Cargo.toml']` and `format: cargo`. The default
sections are `dependencies`, `dev-dependencies` and `build-dependencies`;
`sections` can narrow this list. Legacy Cargo `dev_dependencies` and
`build_dependencies` spellings are inspected with their hyphenated section. The selected sections are also inspected in
all target tables, regardless of host platform. `dependencies` includes the
current manifest's `workspace.dependencies` declarations. Renamed dependencies
are checked using both the local key and the `package` name, as specified by
[Cargo](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html).
Optional dependencies are included even when their feature is disabled.
Inherited `workspace = true` dependencies, path/git/custom-registry sources,
and patch/replace tables remain unverified: this manifest-local check does not
resolve workspace roots, local packages or effective source replacement.

```yaml
- id: rust-packages
  kind: dependencies
  reason: Keep application and build dependencies explicit.
  scope: ['**/Cargo.toml']
  dependencies:
    format: cargo
    allow: [serde, serde_json, cc]
```

Invalid TOML, duplicate keys and incorrectly typed dependency declarations are
unverified. The parser bounds array/inline-table nesting at 10,000 levels;
repository capture also bounds file sizes. These checks are dependency-name
policies, not complete Python or Cargo manifest validators.

These rules inspect manifest declarations. They do not install dependencies,
resolve a complete graph, inspect lockfiles, audit vulnerabilities or enforce
version ranges. A change to a permitted package's version is not itself a
dependency-name violation.

## JSON and YAML values

`document.selector` is a JSON Pointer-style path. `/` separates components;
`~1` represents a literal `/`, `~0` represents `~`, and `*` selects children of
an object or array. Arrays also support numeric indexes. Each document in a
multi-document YAML file is checked.

Choose exactly one condition:

| Condition | Behavior |
| --- | --- |
| `required: true` | The selected value must exist and must not be null. |
| `equals: VALUE` | The selected value must exist and equal the scalar or null value. |
| `forbidden-values: [...]` | An existing selected scalar must not equal a listed value. Missing values are not forbidden by this condition. |
| `allowed-values: [...]` | The selected value must exist and equal one of the listed scalars or null. The list must be nonempty. |
| `pattern: REGEX` | The selected value must exist, be a string and match the Go/RE2 expression. Use `^` and `$` to require a whole-string match. |

```yaml
- id: non-root-containers
  kind: document
  reason: Application containers must explicitly run as a non-root user.
  scope: [deploy/*.yaml]
  document:
    selector: /spec/template/spec/containers/*/securityContext/runAsNonRoot
    equals: true
```

`allowed-values` and `pattern` require v0.4.0 or newer. They are
positive assertions: missing fields, arrays and objects are violations. Null
passes only when explicitly listed in `allowed-values`; regexes never coerce
numbers, booleans, null or YAML timestamps into strings. Empty patterns and
invalid regexes are rejected when loading the contract. For example:

```yaml
document:
  selector: /environment
  allowed-values: [staging, prod]
```

A separate rule can use `pattern: '^v[0-9]+\.[0-9]+\.[0-9]+$'` to constrain
a selected version string. Each rule still has exactly one condition.

String and boolean values remain distinct: `"true"` does not satisfy `true`.
Objects and arrays are not accepted as `equals`, allowed or forbidden values. File
extensions must be `.json`, `.yaml` or `.yml`. Invalid documents, unsupported
YAML constructs and duplicate keys produce diagnostics.

Numbers must retain their decimal value when decoded; silent rounding is not
accepted as equality. Floating-point literals must be finite and at most 1 KiB;
any explicit decimal exponent must be between -400 and 400. Ordinary values
such as `0.1` are supported; arbitrary precision is not. These parser limits
apply to the whole document, including unselected fields. Unsupported numbers
make a document check unverified or a contract invalid. Quote a numeric-looking
value only when it is intended to be a string.

The rule checks committed configuration, not an effective deployment, a
running service or actual dataset rows. It is not a complete JSON Schema,
OpenAPI or Kubernetes validator.

## Project commands

A command rule runs only when the invocation includes `--allow-exec`. Otherwise
it is unverified. Use commands for existing tests and specialist analysis.

```yaml
- id: data-invariants
  kind: command
  reason: The fixed fixture must satisfy the transformation's invariants.
  scope: [data/fixture.csv, scripts/check_data.py]
  command:
    argv: [python3, scripts/check_data.py, data/fixture.csv]
    policy-files: [scripts/check_data.py]
    timeout: 30
    format: exit
    success-codes: [0]
    failure-codes: [1]
```

`argv` is passed as executable and arguments, without implicit shell expansion.
If you explicitly invoke a shell, its behavior is your command's behavior.
Tools must already be available on `PATH`; Linebind does not install them.

| Command field | Default and meaning |
| --- | --- |
| `argv` | Required, nonempty executable/argument list, at most 128 entries. |
| `timeout` | 30 seconds; allowed range 1–300 seconds. |
| `format` | `exit`, `dependency-cruiser` or `conftest`; defaults to `exit`. |
| `success-codes` | `[0]`; at least one code is required. |
| `failure-codes` | `[1]`; may be empty. |
| `policy-files` | Exact repository-relative files from the effective policy source: base for `check`, candidate for `preview`; no globs. |

Success and failure codes must be distinct integers from 0 through 255.
An unexpected exit, timeout, output overflow or malformed structured response
is unverified.

`scope` declares the inputs the check covers. The command receives a disposable
copy of the captured repository, not just the matched files. Listed
`policy-files` use their base versions in `check` and candidate versions in
`preview`. Include
the configuration, policy and helper files that define the check. Unlisted
files are candidate content; Linebind does not discover transitive policy
dependencies for you.

For `format: exit`, only the declared process outcome is established. A failure
does not become an invented file-level finding. This format supports only
`compare: strict`.

The structured formats read the tools' JSON output and preserve their findings.
They support `compare: new` when both analyses are complete. Keep `scope`
aligned with files actually analyzed by the external tool. For example, do not
include a README or tool configuration in an analysis scope if the tool does
not report it as analyzed. Use `policy-files` for the check's configuration.
Missing file coverage, unresolved dependency-cruiser inputs, and Conftest
results with no evaluated rules or skipped/exception results are unverified.
Conftest warnings are retained as findings alongside failures.

Dependency-cruiser can recover from syntax errors without reporting them.
A passed graph check therefore establishes only the declared rules over the
graph it returned. Keep a separate required compiler or syntax check, such as
`tsc --noEmit` or `node --check`, when syntax validity matters to the decision.
See the [examples](../examples/README.md) and [trust model](trust-model.md#command-execution)
for the execution boundary.
Runnable recipes are provided for [dependency-cruiser](../examples/dependency-cruiser/README.md)
and [Conftest](../examples/conftest/README.md).

## Documentation and repository organization

Linebind has no dedicated native rule for forbidden paths, file counts, line
limits or byte budgets. [`require-files`](#require-files) preserves named entry
points but permits additional files. `scope` selects inputs; it does not prohibit
files elsewhere. [`co-change`](#co-change) requires an accompanying edit, even
whitespace, rather than establishing its usefulness.

For documentation growth, separate the decision from what can be verified:

| Responsibility | Example |
| --- | --- |
| Agent instructions | Extend an existing guide when it covers the subject; keep temporary session reports outside the repository; explain the reader need for a new public page. |
| Mechanical checks | Verify required files with native rules, or explicit path and size constraints with a project-owned checker. |
| Human review | Decide whether information is useful, duplicated, accurate or better placed in another guide. |

Instructions guide agents that load and follow them; they are not an executable
gate. A file-count limit can miss duplicated prose inside an existing guide and
can require a policy change for a useful new page. Do not remove useful
explanations or tests just to meet a number. A byte budget tied to a distribution
or performance constraint measures that resource, not overall code quality.

A project-owned checker can enforce agreed paths or budgets through a
[`command`](#project-commands) rule. Specify its actual measurements and
exceptions, exercise permitted changes and violations, and apply the command's
`policy-files` requirements to every helper and settings file controlling the
verdict. Reuse the execution and policy-source rules above; an unlisted helper
remains candidate content. Commands need `--allow-exec`, so they are unavailable
in the [native-only Cloud workflow](cloud-pilot.md).

The checker sees [captured inputs](trust-model.md#captured-inputs), not the full
working directory or Git history. Ignored untracked files are excluded, and
[capture limits](trust-model.md#capture-and-execution-limits) can prevent a
complete check. Probe omitted inputs and new files outside the intended scope;
a green result does not establish documentation quality.

## Compare existing debt

`compare: strict` reports every current violation. Its findings have evolution
`not_comparable`; it does not compute a before/after classification.

`compare: new` evaluates both snapshots using the same effective rule: the base
rule for `check`, the proposed rule for `preview`. It matches findings by
fingerprint, preserving the number of occurrences:

- A matching occurrence is `existing`.
- An additional occurrence is `new` and crosses the check.
- A base occurrence absent from the candidate is `resolved`.

A check with only existing or resolved findings passes under this mode. It is
not evidence that existing debt is acceptable forever. If either analysis is
incomplete, findings cannot be reliably classified and the comparison is
unverified. There is no separately editable baseline file.

Since v0.5.0, a base with no matching files is an empty baseline for `forbid-text`,
`dependencies` and `document`. A first conforming file can pass; its violations
are new. This applies only to known absence: omitted, oversized, unreadable or
unsupported base inputs still prevent comparison, with a `Base:` diagnostic.
An empty candidate scope remains unverified. Command rules still require a
complete base execution; `require-files` still reports missing files.

Native fingerprints include the rule's predicate, file and relevant evidence.
Text identities include the matched line context and occurrence; line-number
movement or indentation alone does not create a new finding. Internal line
reformatting can produce a new finding: text checks do not normalize language
syntax. Renames or changes to evidence
can produce new findings. Structured tool messages also participate in their
identity, so pin tool versions when comparing runs.

For data tests and commands, comparable results also depend on the inputs and
environment. Linebind records repository inputs; it cannot make an external
database or network service identical across runs.

## Status and command-line reference

`required: false` makes a rule advisory without hiding its result. Contract and
declared policy-file changes always require review, independently of a rule's
required/advisory setting. The [README](../README.md#read-the-result-correctly)
explains check statuses and exit codes.

```text
linebind setup --agent codex|claude|antigravity|all [--repo DIR] [--dry-run]
linebind init [--repo DIR] [--contract .linebind.yml]
linebind validate [--repo DIR] [--contract .linebind.yml]
linebind check --base REF [--head REF|worktree] [--repo DIR]
               [--contract .linebind.yml] [--format text|json]
               [--agent] [--output PATH] [--html PATH] [--allow-exec]
linebind preview --base REF [same options as check]
linebind report INPUT.json [--output .linebind/report.html]
```

The default candidate is `worktree`; `--base` is required.

Starting with v0.4.0, `--base merge-base:REF` is also accepted. This explicitly
selects the unique best common ancestor of `REF` and the candidate commit
(`HEAD` for a working-tree candidate). An explicit `--head` is respected even
when it differs from the checkout. The report records the resolved full SHA.
For example, `linebind preview --base merge-base:main` compares local changes
with their branch point. This [Git ancestry calculation](https://git-scm.com/docs/git-merge-base)
does not establish policy approval: `check` reads the ancestor's contract,
which may be older than the current target branch's policy. Use it only when
your repository explicitly chooses that ancestor as its policy source.

Missing references, unrelated histories, multiple best ancestors and shallow
repositories return exit 2; there is no fetch or fallback. CI should keep using
explicit event SHAs. Agents should resolve the agreed base once and reuse its
full SHA throughout a task, rather than recomputing a moving merge base.
 `--agent` adds input
and contract identifiers to text diagnostics and cannot be combined with
`--format json`. `--output` writes the selected
text or JSON format; `--html` writes a self-contained HTML report. `report`
accepts JSON only and renders it without re-running its checks. Its default
output is `.linebind/report.html`. Output paths are relative to the invocation's
working directory, not to `--repo`; use distinct paths for `--output` and `--html`.

An explicit output path may replace a recognized Linebind report of the same
format. An unrelated existing file or non-regular output is rejected. Report
writes use a temporary sibling and rename; there is no `--force` flag.

Reports are snapshots: editing a source file does not update an existing HTML
file. Re-run `check` to obtain a new result. See [the offline report](report.md)
for navigation, copying and interaction checks.
