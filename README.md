<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/wordmark-dark.svg">
    <img src="docs/assets/wordmark.svg" width="280" alt="Linebind">
  </picture>
</p>

<p align="center"><strong>Your repository. Your contract.</strong></p>
<p align="center">Keep the decisions that matter in a versioned contract.<br>Check changes locally, in CI, and in a readable, offline report.</p>

<p align="center">
  <a href="#get-started">Get started</a> ·
  <a href="#a-frontend-change-in-practice">A real workflow</a> ·
  <a href="docs/configuration.md">Write a contract</a> ·
  <a href="docs/examples.md">Try the examples</a>
</p>

<br>

![An illustrated Linebind report showing a dependency outside the approved contract](docs/assets/report-desktop.png)

<p align="center"><sub>A real example result, illustrated on a desktop. <a href="docs/assets/README.md">Artwork and evidence</a></sub></p>

## Keep the reasoning with the rule.

A new dependency. A missing API operation. A deployment setting changed in a
routine refactor. Whether a person or an agent made the change, the question is
the same: does it still respect the decisions your project relies on?

Linebind reads a contract from a chosen Git base and checks a candidate commit
or working tree against it. Each rule has a reason, a scope and a verification
method. The result keeps the evidence, the comparison and the limits together.

- **Make the boundary explicit.** Commit a `.linebind.yml` alongside your code.
  Start with one important decision and add checks when they earn their place.
- **See what changed.** Compare findings with the base to distinguish new,
  existing and resolved issues. Changes to the contract itself stay visible.
- **Understand the result.** Read a compact terminal diagnostic, consume JSON,
  or open a self-contained HTML report. A check that could not run has its own
  status.

Native checks inspect files, text, dependency manifests and JSON/YAML documents.
Opt-in commands connect specialist tools. The same contract workflow works
across frontend, backend, data and infrastructure repositories; the coverage
comes from the checks you actually declare.

## Get started

Download a standalone binary from the [releases](https://github.com/mchl-schrdng/linebind/releases)
for macOS, Linux or Windows, on Intel/AMD or ARM64. Extract the archive and put
`linebind` (`linebind.exe` on Windows) on your `PATH`.
See [installation](docs/installation.md) for archive names and checksum checks.

Linebind requires Git. The distributed binary runs without Go. For a source build:

<details>
<summary>Build this checkout with Go 1.26 or newer</summary>

```sh
git clone https://github.com/mchl-schrdng/linebind.git
cd linebind
go build -o ./bin/linebind ./cmd/linebind
./bin/linebind --version
```

Add the binary's directory to your `PATH`, or use its full path. On Windows,
build to `./bin/linebind.exe`.

</details>

Inside the Git repository you want to check:

```sh
linebind init
linebind validate
```

`init` creates a small starter contract and adds `/.linebind/` to `.gitignore`.
Edit the rules to describe your project, validate them, then review and commit
the contract. The base used below must contain that reviewed contract.

```sh
linebind check --base HEAD --html .linebind/report.html
```

This checks the current working tree against the contract in `HEAD`. Open
`.linebind/report.html` in your browser when you want the visual report; Linebind
does not start a server or open a browser automatically.

For a commit comparison and a machine-readable result:

```sh
linebind check --base main --head HEAD --format json --output .linebind/report.json
linebind report .linebind/report.json --output .linebind/report.html
```

Both revisions must already be available locally. Pick the base deliberately:
Linebind uses the revision you provide, rather than deciding which change your
team has approved.

## A frontend change in practice

You ask an agent to add a product carousel to a React storefront. It adds jQuery
to implement the feature. The team's decision was to review every new runtime
dependency, so the approved contract on `main` contains:

```yaml
version: 1
name: Frontend runtime dependencies

rules:
  - id: frontend.runtime-dependencies
    kind: dependencies
    reason: The browser bundle uses React; new runtime packages require a separate decision.
    scope: [package.json]
    dependencies:
      format: npm
      sections: [dependencies]
      allow: [react, react-dom]
    remediation: Remove the new runtime dependency or propose a reviewed contract change.
```

On the feature branch, `package.json` changes:

```diff
 "dependencies": {
   "react": "19.0.0",
-  "react-dom": "19.0.0"
+  "react-dom": "19.0.0",
+  "jquery": "3.7.1"
 }
```

With that reviewed contract already committed on `main`, check the feature
branch's working tree:

```sh
linebind check --base main --html .linebind/report.html
```

![Actual Linebind CLI output: jQuery violates the reviewed frontend dependency contract, exit 1](docs/assets/frontend-cli.png)

<p align="center"><sub>Actual v0.1.0 stdout, rendered as an image with line wrapping. <a href="docs/assets/frontend-cli.txt">Plain text</a> · <a href="docs/assets/frontend-cli.json">JSON evidence</a> · <a href="docs/assets/README.md#cli-output">Capture notes</a></sub></p>

The check returns `crossed` and exits with `1`. Its evidence identifies the
dependency, the reason for the rule and the next action. Adding jQuery to the
candidate's allowlist does not authorize it: this run still uses the contract
from `main` and reports the policy change separately.

The team can remove the dependency or review a contract change. This rule checks
declared dependency names in `dependencies`; it does not judge carousel behavior,
package versions or vulnerabilities. `Not comparable` in this output means the
rule uses the default strict mode; `compare: new` enables finding evolution.

The repository includes the [exact fixture](examples/frontend/base/.linebind.yml).
To reproduce its passing, crossed, malformed-input and removed-contract states,
use Go 1.26+ and an installed binary, then run
`go run ./scripts/pilots -binary /path/to/linebind` from this checkout.
No application dependencies are installed or executed.

The [runnable examples](docs/examples.md) cover four different contexts:

| Context | A decision to preserve | What the native example checks |
| --- | --- | --- |
| Frontend | Keep runtime dependencies intentional | Names declared in an npm manifest |
| Backend | Keep an API operation and response declared | Selected values in an OpenAPI document |
| Data | Keep a field's declared nullability | A committed table-schema document |
| Infrastructure | Require a non-root container setting | A value in a Kubernetes manifest |

These are focused examples. They do not install packages, call a live API,
validate dataset rows or apply infrastructure.

## In a pull request

The same check can run before a build or deployment. The complete
[GitHub Actions workflow](.github/workflows/contract.yml) pins the published
Linebind binary and its checksum, loads the reviewed base commit, and checks the
PR's exact head commit. JSON remains visible in the job log; JSON and HTML are
retained as downloadable artifacts even when the check fails.

Its core command uses explicit revisions from the pull-request event:

```sh
linebind check --base "$BASE_SHA" --head "$HEAD_SHA" \
  --format json --html "$RUNNER_TEMP/linebind-report/report.html" \
  | tee "$RUNNER_TEMP/linebind-report/result.json"
```

The workflow uses Bash with `pipefail`, so an exit code of `1` or `2` fails the
job. Make **Linebind contract** a required check in your branch rules to gate
merges, or add `needs: contract` to a build job in the same workflow. A reviewed
merge can then trigger your existing deployment pipeline.

See [CI integration](docs/ci.md) for the copyable workflow, first-contract setup,
policy-change review, artifact access and the workflow's trust boundary.

## How the check works

**No LLM, API key or model service is involved.** Linebind is a Go executable:

1. Git resolves the chosen base and captures the candidate files.
2. Linebind parses the base's `.linebind.yml` and selects the declared inputs.
3. Native rules inspect those captured bytes. In the frontend example, it parses
   `package.json` and compares dependency names with the approved allowlist.
4. It records findings, policy changes and incomplete checks, then produces the
   exit code, terminal/JSON output and offline HTML report.

`reason` and `remediation` explain a rule to people and agents; the check is
defined by `kind`, `scope` and its parameters. Free-form prose is not interpreted
as an executable rule. With the same captured inputs and contract, native checks
produce the same findings; timestamps and execution durations can differ.

Native checking makes no model calls and uploads no source code. Optional
`command` rules require `--allow-exec` and invoke the external tool you configure;
that tool may have its own network or credential requirements.
A coding agent can invoke the same CLI and read its JSON or `--agent` diagnostics;
Linebind does not need a connection to the agent's model provider.

## The contract changes too.

The base supplies the effective contract and any explicitly declared policy
files. Editing or removing them in the candidate does not replace the rules
used for that run. The report shows the change and asks for review.

Legitimate changes belong in the review process: explain the new decision,
review the affected rules and policy files, then adopt the approved revision as
the next base. v0.1.0 does not interpret a PR label or an approval field as
authorization. The [trust model](docs/trust-model.md) describes the complete
boundary and the policy-change workflow.

For an existing repository, `compare: new` checks for additional findings while
keeping existing debt visible. It compares both snapshots under the same rule;
it does not mark the whole repository clean.

## Read the result correctly.

| Check status | Meaning |
| --- | --- |
| `passed` | The declared check passed under its comparison mode. |
| `crossed` | The check found a violation or a declared command returned a failure outcome. |
| `unverified` | The available inputs or execution could not establish a complete result. |

Required checks determine the command's exit code. Advisory findings remain in
the report, so a report can show a crossed or unverified advisory check while
the CLI exits successfully.

| Exit code | Meaning |
| --- | --- |
| `0` | Required checks passed and no effective policy change requires review. |
| `1` | A required check crossed its boundary, or the contract/policy files changed. |
| `2` | Verification is incomplete for a required check, or the command/input is invalid. |

When a required violation and an incomplete check coexist, the exit code is `1`;
the incomplete check and its diagnostics remain visible.

Native checks do not execute project code. A `command` rule stays unverified
unless you pass `--allow-exec`. Enabled commands run with your host permissions
in a disposable copy; that copy is **not a sandbox**. See
[command execution](docs/trust-model.md#command-execution) before enabling it in CI.

## A few honest questions

**Why not use my existing linter and scripts?**

Keep them. Linebind adds a shared place for the decision, its reason, its scope,
the base/candidate comparison and the evidence. If your current setup already
makes that clear with less work, another tool may not help.

**Does a passing result mean the repository is correct or secure?**

It means the required, declared checks passed for these inputs. Unchecked
behavior, visual design, business intent and runtime conditions still need
their own verification and review.

**Can it check any language?**

The contract and report are language-independent. Native checks have specific
formats and limits; specialist analysis depends on the external tool you
configure. A lexical pattern is not an import graph or an AST rule.

**Does it need an account or a model API?**

No. The CLI and HTML report work locally. External commands may have their own
network, account and environment requirements.

<details>
<summary><strong>Development and release information</strong></summary>

```sh
go test ./...
go vet ./...
```

The CLI is written in Go. Native checking and report rendering need no
frontend runtime. The HTML report embeds its styles and script, and its content
remains readable without JavaScript.

[Contributing](CONTRIBUTING.md) · [Configuration](docs/configuration.md) ·
[Trust model and limits](docs/trust-model.md) · [Examples](docs/examples.md) ·
[Validation](docs/validation.md)

</details>

<br>

---

<p align="center">
  Made by <a href="https://github.com/mchl-schrdng">mchl-schrdng</a> ·
  <a href="LICENSE">MIT</a> ·
  <a href="SECURITY.md">Security</a>
</p>
