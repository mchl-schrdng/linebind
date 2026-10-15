# What a Linebind result establishes

Linebind evaluates declared checks against captured repository inputs. A result
identifies the base, candidate, input digest and contract hash. These identifiers
make the run traceable; they are not signatures or proof that an operator used
a trusted executable.

The base is supplied by the caller. Linebind resolves it to a local Git commit;
it does not determine whether that commit was reviewed or authorized. Your
repository and CI process establish that trust.

## The effective contract in `check`

The selected base supplies:

- The contract at the chosen repository-relative path.
- Every file named in a command rule's `policy-files`.

The candidate supplies the source and configuration being checked. Native rules
use the contract loaded from the base. For command checks, declared policy
files from the base are overlaid into the disposable candidate copy.

Candidate changes to these policy files are reported separately and require
review. The current run continues using the base policy. Removing a rule from
the candidate contract therefore does not remove it from that run.

Policy comparison ignores CRLF/LF line-ending differences in UTF-8 text, so a
Git checkout on Windows does not itself propose a policy change. Other
formatting or comment edits still require review. A policy file's executable-bit change is
also a policy change. Renaming a policy file appears as removal of the effective
path; a newly added file is not automatically adopted as its replacement.

Only explicitly listed policy files receive this treatment. If a tool's
configuration imports other files, declare those files too. Keep the verifier,
tool versions, CI workflow and chosen base under an appropriate review process.

## Evolving the contract

Updating the rules is a normal project decision:

1. Explain the intended change and its effect on existing findings.
2. Review the contract and every affected policy file.
3. Use your repository's designated maintainer process to approve that policy
   change and incorporate it into the trusted branch.
4. Use the approved revision as the base for later code checks.

Our [PR workflow](ci.md) validates the candidate with `preview`. A valid policy
change does not fail merely because it differs from the base. Required
violations, incomplete required checks and invalid contracts still fail. The
maintainer reviews the policy diff and merges only after all required CI checks
pass; that merge adopts the new contract. No second reviewer or exception path
is required for this single-maintainer repository.

`check` remains available for enforcing an already adopted policy: a policy-only
change returns exit `1` against the old base. Repositories choosing that stricter
CI mode need their own adoption process. Linebind has no approval-label API or
automatic bypass in either mode. Do not select the candidate as its own base
merely to manufacture compliance with the old contract.

The first contract also needs this bootstrap review: `init` writes a starter;
`validate` checks its schema; neither operation makes it approved. `check`
requires the selected base to already contain a valid contract.

## Preview a proposed contract

`preview` is a separate analysis, not an alternate approval path. It captures
the same base and candidate inputs, but loads the contract and declared policy
files from the candidate. The base need not contain a contract. For
`compare: new`, both snapshots are evaluated under the same proposed policy.
Policy differences are displayed but do not alone make preview exit `1`.

Every preview report carries `purpose: preview`; terminal output, HTML and
copied diagnostics identify it as not approved. A successful preview establishes
only the proposed checks' result. The normal `check` against the approved base
still reports the policy change. Preview can be a required candidate-policy
validation in CI; it cannot prove compliance with the old policy or approve
the new one. That distinction remains visible in all reports.

Preview commands require `--allow-exec` just like check commands. Unlike check,
their declared policy files come from the candidate and may be unreviewed.
Review the invocation and execute in an appropriate isolated environment.
Installing the bundled agent skill does not grant execution permission, approve
a policy or change this trust boundary. See the [agent workflow](agents.md).

## Captured inputs

A commit comparison reads Git objects at the resolved revisions. A working-tree
comparison captures tracked files and untracked, non-ignored files. Ignored
untracked files are not part of the snapshot; tracked files remain inputs even
if an ignore pattern matches them. A working tree needs an initial commit.

The snapshot does not depend on whether a tracked edit has been staged. Deleted
tracked files are absent from the candidate. The input digest incorporates
captured file paths, contents, executable bits and recorded omissions. On Windows,
tracked executable flags come from the Git index because the filesystem does
not expose the Unix executable bit.

Linebind does not automatically fetch revisions, resolve Git LFS objects,
initialize submodules or follow symlinks. Missing Git objects and an unavailable
base leave verification incomplete. Use a stable working tree: a file detected
as changing during capture causes a retryable error. This is a bounded capture,
not a filesystem transaction across the entire repository.
Stop concurrent source writes when reviewing a working-tree result. For a
stable release or review comparison, use a committed candidate with
`--head FULL_COMMIT_SHA`.

## Native checks

Native checks read the snapshot in memory. They do not run package installation,
project build scripts, tests or code from the candidate. Git is used to obtain
repository inputs.

Their scope is intentionally specific:

| Check | What it does not establish |
| --- | --- |
| Required files | The correctness or usefulness of their contents |
| Forbidden text | Language semantics, an AST match or an import boundary |
| Dependency names | The full resolved graph, lockfile integrity, acceptable versions or absence of vulnerabilities |
| Document values | Runtime behavior, full schema validity or actual data quality |

An omitted input matching a check's scope makes that check unverified. Findings
and diagnostics can coexist: read the diagnostics as well as the visible
violations. `compare: new` requires complete base and candidate analyses before
classifying findings as new, existing or resolved.

## Command execution

Command checks require the explicit `--allow-exec` flag. Without it, they remain
unverified. This applies to both arbitrary commands and the structured
dependency-cruiser/Conftest integrations.

An enabled command runs in a temporary copy of the captured source, with the
base versions of its declared policy files in `check`, or candidate versions
in `preview`. No `.git` directory is materialized.
Linebind supplies temporary home and temporary-directory locations and a small
environment including `PATH` and required platform variables. It does not pass
the calling shell's full environment to the process.

After execution, captured source files are checked for changes to their contents
and, on platforms that preserve it, executable status. A command that changes
those inputs cannot supply a complete verdict. The temporary copy is removed
afterward. Commands may create derived files there; they are not published back
to the checkout.

**A disposable copy and a reduced environment are not a sandbox.** The process
still has the invoking user's host permissions and may access the filesystem,
network or other resources available to that user. A project command, helper or
configuration file can execute arbitrary code. Permission to run the command
also covers the code it invokes.

For untrusted changes, use a separately isolated execution environment with
appropriate filesystem, network and credential restrictions. Provision and pin
the required tools there. Linebind neither installs tools nor establishes that
the executable found on `PATH` is an approved version.

The `exit` format establishes only the configured exit-code outcome. The two
structured integrations interpret the tool's JSON evidence; they do not
independently re-run its analysis. Missing or unsupported evidence, unresolved
inputs and failed analysis must remain visible as incomplete verification.

Tool limitations also constrain the result. Dependency-cruiser can recover from
syntax errors without reporting them, so a passed check means its reported
graph satisfies the declared rules; it does not establish syntax validity.
Use a separate required compiler or syntax check for that evidence.

## Capture and execution limits

| Resource | Limit or behavior |
| --- | --- |
| Individual captured file | 4 MiB |
| Captured content per snapshot | 128 MiB |
| Files per snapshot | 25,000 |
| Contract file | 1 MiB |
| Rules per contract | 256 |
| Native findings per check | 1,000; truncated results are reported as incomplete |
| Command timeout | 30 seconds by default; maximum 300 seconds |
| Command stdout | 8 MiB |
| Command stderr | 64 KiB |
| Included command failure log | Up to 4 KiB |

Symlinks, submodules, special files, Git LFS pointers and oversized files are
not inspected as source contents. Omissions are recorded and affect checks
whose declared scope includes them. Command execution requires a complete
source snapshot and refuses a snapshot containing such omissions. Aggregate
capture limits can prevent the whole analysis from running.

The snapshot limits apply to the captured repository, including files outside
any selected rule scope. Narrowing a rule's scope does not reduce that capture.
The repository capture limit takes precedence over a parser's own larger limit.
There is no promise that every monorepo, generated tree or binary-heavy
repository fits these bounds. Keep input selections specific and read any
omission diagnostics.

YAML parsing rejects aliases, anchors, duplicate keys and unsupported tags.
Document nesting and the number of documents are bounded. These are deliberate
format limits, not evidence that a rejected file is an invalid project file.
Numeric parsing also rejects decimal precision loss and out-of-range literals;
see the [document limits](configuration.md#json-and-yaml-values). It does not
silently round distinct values into a passing equality result.

## Reports and writes

`check` captures source inputs without editing the checkout. Reports are
explicit outputs: `--output` writes text or JSON and `--html` writes HTML.
`report` reads a saved JSON result and writes HTML. `init` writes a starter
contract and adds the report directory to `.gitignore`.

Output paths are explicit, except for `report`'s default `.linebind/report.html`.
An existing file is replaced only when recognized as a Linebind report of the
requested format. The CLI refuses unrelated files and non-regular outputs.
Each report is written to a temporary sibling before renaming it into place.
These recognition checks prevent accidental overwrites; they do not
authenticate a report or its author.

Report output does not change the recorded verdict. The HTML embeds its styles
and script and needs no server, account or remote asset. It is a read-only
snapshot, with a copy-diagnostic action. Rendering a saved report does not
revalidate the repository or authenticate the report's author.

Repository values are escaped in HTML. HTML and terminal text remove control
and bidirectional-formatting characters, while preserving newlines and tabs.
Reports may still contain private paths, rule text, finding details and excerpts
from external tools. Review them before sharing them outside the project.

## CI integration

Install a trusted Linebind binary and make the chosen base and candidate Git
objects available locally. Pass explicit revisions to the command, and retain
JSON/HTML output when you need review evidence. Do not use an unreviewed change
to rebuild the verifier or rewrite the policy-enforcement workflow and then
treat its own result as independent verification.

Choose required and advisory checks deliberately. The report preserves both;
`check` enforces required checks plus policy-change review, while `preview`
enforces required checks under the proposed policy. CI must actually use the
selected exit code as a gate. A passing proposal can still remove useful rules:
the maintainer must assess the policy diff, just as changes to tests or CI itself.

A passing result is about these checks and these inputs. It does not certify
the repository, authenticate the author of a change or replace code review.

For vulnerability reporting and release-artifact limitations, see
[SECURITY.md](../SECURITY.md).
