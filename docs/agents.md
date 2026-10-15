# Working with coding agents

Linebind has two parts: a standalone verifier and a bundled skill that teaches
an agent when to use it. The CLI performs the declared checks. The skill helps
turn decisions into a contract, correct unintended deviations and prepare
deliberate policy changes. The same CLI also works without an agent.

The consuming project does **not** need to clone Linebind's Go repository.
Install the executable in each development or CI environment that runs checks.
Commit the project's contract and agent instructions alongside its source.
The skill uses your existing agent; Linebind adds no model account or API key.
Your agent's own source-access and service policies still apply.

`setup` and `preview` require Linebind v0.2.0 or a compatible source build;
they are not available in v0.1.0. See [installation](installation.md).

## Install once per repository

After installing the CLI, run this in the target Git repository:

```sh
linebind setup --agent all --dry-run
linebind setup --agent all
```

Or select `codex`, `claude` or `antigravity` instead of `all`.
The installation runs offline and uses the skill embedded in that executable.
It does not install project dependencies, configure hooks, change permissions,
choose a trusted base or create a contract.

| Target | Skill location | Repository instructions |
| --- | --- | --- |
| Codex | `.agents/skills/linebind/` | A short block in `AGENTS.md` |
| Claude Code | `.claude/skills/linebind/` | A short block in `CLAUDE.md` |
| Antigravity | `.agents/skills/linebind/` | The same `AGENTS.md` block |

`all` writes two identical copies of the skill, with no symlinks. Existing
instructions are preserved. Repeating the same installation changes nothing.
Git's LF/CRLF conversion is accepted without rewriting existing skill files.
If an existing Linebind skill or instruction block differs, setup stops before
writing: reconcile those files deliberately, including during upgrades. There
is no force-overwrite option. Ordinary destination conflicts are preflighted;
an operating-system error during writing can leave a partial installation,
which the error reports for inspection and retry.

Review and commit the generated files. Reload skills or start a new agent
session if discovery does not refresh. The installation is repository-local;
it does not change your global agent preferences. These locations follow the
official [Codex skills](https://learn.chatgpt.com/docs/build-skills),
[Claude Code skills](https://code.claude.com/docs/en/skills), and
[Antigravity skills](https://antigravity.google/docs/skills) documentation.
Codex and Antigravity support repository `AGENTS.md` instructions. Claude gets
its own small block so an existing `CLAUDE.md` does not hide this guidance.
See [Codex instructions](https://learn.chatgpt.com/docs/agent-configuration/agents-md),
[Claude project memory](https://code.claude.com/docs/en/memory), and
[Antigravity rules](https://antigravity.google/docs/rules/).

## Upgrade installed guidance

The skill is embedded in the executable; upgrading the binary alone leaves
previously committed skill files unchanged. Setup intentionally refuses to
overwrite different content, including an older bundled version.

After installing the new binary, generate its files in a disposable repository:

```sh
GUIDANCE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/linebind-guidance.XXXXXX")
git -C "$GUIDANCE_DIR" init -q
linebind setup --repo "$GUIDANCE_DIR" --agent all
printf 'New guidance: %s\n' "$GUIDANCE_DIR"
```

Compare the generated `.agents/skills/linebind/`, `.claude/skills/linebind/`
and Linebind blocks in `AGENTS.md`/`CLAUDE.md` with your project. Apply the
reviewed changes to the targets you use, preserving custom instructions and
all content outside the Linebind blocks. Commit that diff and reload skills.
`linebind setup --agent all --dry-run` then reports unchanged files if your
installation exactly matches the bundle; intentional customizations can keep
producing a conflict and should be maintained deliberately.

## From a first version to a first contract

During design, keep the decisions and their reasons in your existing brief or
architecture document. Ponytail or another planning workflow can help you make
those choices; the Linebind skill has no dependency on either.

At the first stable milestone, ask your agent:

> Prepare a Linebind contract for this version from our documented decisions.
> Explain what each rule protects and what remains outside automated coverage.
> Show a passing example and a deliberate violation in an isolated copy.

For example, an approved frontend decision might limit production dependencies
to React and React DOM and require ESM configuration. Both can become checks.
“Keep the interface elegant” remains a human review criterion unless you define
an appropriate measurable check. Files and dependencies found in the repository
are observations, not automatically approved architectural decisions.

The agent prepares `.linebind.yml`, validates its syntax and previews its
behavior. With an initial Git commit already present, choose a comparison
revision explicitly. In the examples below, `BASE_SHA` is a full commit ID
selected by your project; the command does not choose it for you.

```sh
linebind validate
linebind preview --base "$BASE_SHA" --agent --html .linebind/preview.html
```

For this bootstrap preview, the comparison revision need not contain a contract.
The candidate supplies the proposed contract. A preview is an experiment, not
approval. Adopt the first contract through the repository's review process
before using its approved revision as the base of a required `check` in CI.
Our candidate-policy CI workflow can validate the first contract before merge;
it does not require a contract in the base. A repository with no initial commit
cannot yet be captured by `preview`.

## Everyday development

For everyday work, follow the repository's selected CI mode. Our workflow
validates the candidate contract with `preview`. To separately establish
compliance with an already adopted contract, use `check`:

```sh
linebind check --base "$BASE_SHA" --agent --html .linebind/check.html
```

Here `BASE_SHA` must identify the repository-approved policy revision. Keep it
fixed for the task; selecting the candidate's own `HEAD` merely to obtain a
pass is not approval. Linebind neither fetches missing revisions nor decides
which revision your team trusts. CI supplies its explicit base and head SHAs.

If an agent adds jQuery to a React-only frontend unnecessarily, it should
correct the implementation and remove that dependency. It should not widen the
allowlist, disable the rule or ask you to approve an ordinary repair.

A required check that could not run remains incomplete. A zero exit code can
still accompany advisory violations or incomplete advisory checks; the agent
must read the individual results and diagnostics. Contract checks complement
the project's tests, build and review, and do not establish correctness outside
the declared rules.

## Deliberately change a decision

For example:

> We have decided to adopt Zod for this form. Implement the validation and
> update the contract to allow that dependency; retain the other boundaries.

This already authorizes preparing that code and contract change. The skill
must not ask the same question again. It should show the narrow policy diff,
its reason and its effect on coverage, then retain both kinds of evidence:

```sh
linebind check --base "$BASE_SHA" --format json --output .linebind/check.json
linebind preview --base "$BASE_SHA" --format json --output .linebind/preview.json \
  --html .linebind/preview.html
```

Run these independently: an expected nonzero `check` must not prevent obtaining
the preview. The first still uses the old contract and returns exit `1` for a
policy change. The second tests the proposal and can succeed without approving
it. For `compare: new`, preview applies the same proposed policy to both source
snapshots; it does not turn the candidate into its own comparison baseline.

If the request does not authorize changing the decision, the agent prepares a
concrete alternative and asks only for the unresolved decision. “Make CI green”
does not by itself authorize weakening the contract.

The repository chooses its CI policy. Our workflow uses `preview` as required
validation of the proposed contract: a valid policy edit alone does not fail,
and the maintainer adopts it by merging after all required checks pass. No
second reviewer is required here. The agent follows that configured workflow;
it must not change the workflow or weaken a rule simply to obtain green.

A repository that chooses `check` as its required gate instead keeps the old
policy authoritative and needs its own policy-adoption process. Neither mode
has an approval-label API or a local `approved: true` switch. See
[CI integration](ci.md#bootstrap-and-policy-changes).

## What preview establishes

| | `check` | `preview` |
| --- | --- | --- |
| Contract and declared command-policy files | From the selected base | From the candidate |
| Base must already contain a valid contract | Yes | No |
| Policy edits automatically require review in the exit result | Yes | No; displayed as proposal differences |
| Exit `0` | Required checks passed with no effective policy change | Required checks passed under the proposed policy only |
| Exit `1` | Required violation or policy change | Required violation under proposed policy |
| Exit `2` | Required verification incomplete or operation failed | The same, for proposed-policy analysis |
| JSON discriminator | No `purpose` field (existing format) | `"purpose": "preview"` |
| CI validates against an already adopted policy | Yes, with a trusted base/process | No |
| CI validates a proposed policy before maintainer adoption | Policy edits fail the gate | Yes; adopted only through the repository process |

Preview HTML, terminal output and copied diagnostics are explicitly labelled
**PREVIEW / NOT APPROVED**. Saved preview JSON retains that purpose when rendered
again. v0.1.0 rejects this new JSON field instead of rendering a preview as an
ordinary check. Reports remain traceable records, not signed authorizations.

Native previews execute no project code. A preview with `--allow-exec` can run
commands and policy code from the **candidate**, which may be unreviewed. The
flag is explicit permission to execute them on that host; use a suitable
isolated environment for untrusted inputs. The disposable copy is not a sandbox.

## Compatibility and boundaries

The same bundled content is installed for all targets. Provider integration
uses documented instruction and skill locations, not a private model API.
Supported directory layouts and CLI behavior can be tested deterministically;
an installed file does not prove that a particular agent session loaded or
followed it. Agent settings, version, permissions and environment still matter.

A real Codex CLI trial loaded the skill and repaired a deviation. The Claude
Code trial stopped on expired authentication before execution; Antigravity
runtime discovery has not been exercised. See the [validation record](validation.md#agent-workflow-v020)
for the five controlled workflow trials and the exact limits of these results.

The skill source lives in
[`internal/agentsetup/skill`](../internal/agentsetup/skill/).
The executable embeds it, so installation needs neither GitHub access nor a
checkout of this repository. Skill discovery guides the agent; the independent
[CI check](ci.md) supplies the enforcement boundary.
