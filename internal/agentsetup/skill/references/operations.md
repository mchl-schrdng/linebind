# Operating Linebind

This skill requires Linebind v0.2.0 or a compatible source build.
`setup` and `preview` are not available in the published v0.1.0 executable.
Use a compatible supplied build or a release that documents these commands;
check `linebind --version` and `linebind --help` before assuming support.

## Prerequisites

- Git, a Git repository, and a compatible Linebind executable on `PATH` or at
  the user-provided path. Go, a cloned Linebind source tree, and a model API key
  are not required to check another repository.
- An initial Git commit for snapshot-based `check` and `preview`. Do not create
  a commit solely to make an unapproved contract appear approved. If the
  repository has no commit, author and validate the draft, and explain that
  snapshot verification needs the initial commit.
- Locally available base and candidate Git revisions. The CLI does not fetch
  missing objects. Retrieve the designated revision through the repository's
  normal workflow when authorized; never replace it with an unrelated commit.

If the executable is missing or too old, explain the exact prerequisite and
refer to [the installation guide](https://github.com/mchl-schrdng/linebind/blob/main/docs/installation.md)
and [releases](https://github.com/mchl-schrdng/linebind/releases). Do not silently
install globally, execute `curl | sh`, or require a source checkout. Continue
drafting or reviewing the contract when useful, without reporting checks as run.

Run commands from the target repository root. If using another directory, pass
`--repo /path/to/repository`; report output paths are relative to the invoking
process, not automatically to `--repo`.

When installing this integration into another repository is requested, use
`linebind setup --agent codex --dry-run` to inspect it, then
`linebind setup --agent codex` to install. Supported agent selections are
`codex`, `claude`, `antigravity`, and `all`. Installation writes repository-local
instructions and this bundled skill; it does not install the executable,
approve a contract, or configure a CI gate. Do not reinstall on every task.

## Select and hold the base

Read the task and repository workflow to identify the agreed reference. Resolve
that actual reference with `git rev-parse --verify 'REFERENCE^{commit}'` and use
the returned full SHA. `REFERENCE` is a placeholder, not a default. Do not infer
that an arbitrary local `main`, current `HEAD`, or a candidate's instructions
establish a new approved base. The CLI records identity; the repository process
establishes trust.

For an initial preview only, an existing commit can serve as the comparison
base even when it contains no contract. Identify it as a comparison baseline,
not as an approval. This does not enable a gate until the trusted base contains
the adopted contract.

## Commands

Replace `BASE_SHA` and `HEAD_SHA` below with the actual resolved revisions.
Omitting `--head` selects the working tree, including non-ignored untracked
files. `--head HEAD_SHA` checks only that commit, excluding uncommitted edits.

```sh
linebind --version
linebind --help
linebind init
linebind validate
```

`init` creates a starter and ignores `/.linebind/`; it refuses to replace an
existing contract. Use it only when creating a first contract. `validate`
checks the local YAML schema without executing any checks. The default contract
is `.linebind.yml`; `--contract path/to/contract.yml` selects another
repository-relative path for these commands, `check`, or `preview`.

```sh
linebind check --base BASE_SHA --agent
linebind check --base BASE_SHA --head HEAD_SHA --format json --output .linebind/check.json --html .linebind/check.html
linebind preview --base BASE_SHA --agent
linebind preview --base BASE_SHA --format json --output .linebind/preview.json --html .linebind/preview.html
linebind report .linebind/check.json --output .linebind/check.html
```

`--agent` is compact text; do not combine it with `--format json`. Preserve the
command's actual exit code and inspect its report even on a nonzero exit.

**Check** loads the contract and declared command policy files from the approved
base. Candidate policy edits are reported for review and do not replace those
rules during the run.

**Preview** loads the candidate contract and declared policy files. It uses the
base only for comparison, including `compare: new`. Its report has
`purpose: preview`. It is a draft evaluation, even when all checks pass. It does
not replace `check`, approve the policy, or supply an approved CI gate.

For a policy change, retain separate check and preview reports. For the first
contract, report that the gate is not yet available if the approved base lacks
a contract; the preview can still evaluate the draft.

## Interpret the evidence

| Evidence | Meaning and response |
| --- | --- |
| `passed` | The declared check passed for the captured inputs and comparison mode. |
| `crossed` | Inspect the finding and reason; fix an accidental code violation or prepare the explicitly intended policy change. |
| `unverified` | Verification is incomplete. Read the diagnostic; do not invent an architectural defect or a passing result. |
| Policy changes in `check` | Review is required. Editing the candidate contract does not authorize itself. |

For `check`, exit `0` means required checks passed with no policy change;
`1` means a required violation or policy change; `2` means a required incomplete
check or operational error. Required violations take precedence over incomplete
checks. Advisory checks can be crossed or unverified even when the exit is `0`.
Preview exit codes describe its draft evaluation and never establish approval.

Record the report's base, candidate, input digest, and contract hash when sharing
evidence. A report is a snapshot, not an ongoing monitor. A modified working tree
needs another run; a changed approval decision needs a new reviewed base.

## External checks

Native checks require no project dependency installation or project execution.
`command` rules require already available tools and the opt-in `--allow-exec`.
Read the command and the files it invokes before using existing execution
authorization; otherwise obtain the needed permission without re-asking for
authorization already given. Preview executes candidate policy when enabled.
The disposable copy is **not a sandbox**: commands retain host permissions.

Missing tools, timeouts, invalid output, or unsupported inputs remain unverified.
Do not silently make a command optional to get green. Use the repository's
documented isolated environment for untrusted command execution. Linebind does
not provision tools or grant execution permission itself.
