# The repository contract

Linebind reads a versioned YAML contract. The default path is `.linebind.yml`;
`--contract` selects another repository-relative path. `validate` checks the
local file's syntax and schema. `check` evaluates the contract stored in the
chosen base revision.

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
| `kind` | One of `require-files`, `forbid-text`, `dependencies`, `document`, `command`. |
| `reason` | Required explanation of the project decision. |
| `scope` | Repository-relative globs selecting files. Required except for `require-files`. |
| `required` | Defaults to `true`. Set `false` for an advisory check. |
| `compare` | `strict` by default; `new` compares findings with the base. |
| `remediation` | Optional next step shown with findings. |

Use exactly one configuration block matching `kind`: `files`, `text`,
`dependencies`, `document` or `command`.

Paths use `/` on every platform. Globs support `**`, for example `src/**/*.ts`.
They must be relative to the repository root: no absolute paths or `..`
traversal. Except for `require-files`, a scope matching no files is unverified.
An empty selection is not a successful check.

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

```yaml
- id: non-root-containers
  kind: document
  reason: Application containers must explicitly run as a non-root user.
  scope: [deploy/*.yaml]
  document:
    selector: /spec/template/spec/containers/*/securityContext/runAsNonRoot
    equals: true
```

String and boolean values remain distinct: `"true"` does not satisfy `true`.
Objects and arrays are not accepted as `equals` or forbidden values. File
extensions must be `.json`, `.yaml` or `.yml`. Invalid documents, unsupported
YAML constructs and duplicate keys produce diagnostics.

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
| `policy-files` | Exact repository-relative files to load from the base, without globs. |

Success and failure codes must be distinct integers from 0 through 255.
An unexpected exit, timeout, output overflow or malformed structured response
is unverified.

`scope` declares the inputs the check covers. The command receives a disposable
copy of the captured repository, not just the matched files. Listed
`policy-files` are replaced by their base versions before execution. Include
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
See the [examples](examples.md) and [trust model](trust-model.md#command-execution)
for the execution boundary.
Runnable recipes are provided for [dependency-cruiser](../examples/dependency-cruiser/README.md)
and [Conftest](../examples/conftest/README.md).

## Compare existing debt

`compare: strict` reports every current violation. Its findings have evolution
`not_comparable`; it does not compute a before/after classification.

`compare: new` evaluates the base and candidate using the same base rule. It
matches findings by fingerprint, preserving the number of occurrences:

- A matching occurrence is `existing`.
- An additional occurrence is `new` and crosses the check.
- A base occurrence absent from the candidate is `resolved`.

A check with only existing or resolved findings passes under this mode. It is
not evidence that existing debt is acceptable forever. If either analysis is
incomplete, findings cannot be reliably classified and the comparison is
unverified. There is no separately editable baseline file in v0.1.0.

Native fingerprints include the rule's predicate, file and relevant evidence.
Text identities include the matched line context and occurrence; line-number
movement alone does not create a new finding. Renames or changes to evidence
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
linebind init [--repo DIR] [--contract .linebind.yml]
linebind validate [--repo DIR] [--contract .linebind.yml]
linebind check --base REF [--head REF|worktree] [--repo DIR]
               [--contract .linebind.yml] [--format text|json]
               [--agent] [--output PATH] [--html PATH] [--allow-exec]
linebind report INPUT.json [--output .linebind/report.html]
```

The default candidate is `worktree`; `--base` is required. `--agent` adds input
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
