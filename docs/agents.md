# Working with coding agents

The bundled skill guides your existing coding agent through creating, checking
and evolving a repository contract. The CLI evaluates the declared checks; it
also works without an agent. Linebind adds no model account or API key.

Install the executable in each environment that runs checks. Consuming projects
do not need Linebind's source repository or Go. `setup` and `preview` require
v0.2.0 or newer. See [installation](installation.md).

## Install once per repository

From your project:

```sh
linebind setup --agent all --dry-run
linebind setup --agent all
```

Select `codex`, `claude` or `antigravity` instead of `all` to install one target.

| Target | Skill location | Repository instructions |
| --- | --- | --- |
| Codex | `.agents/skills/linebind/` | A short block in `AGENTS.md` |
| Claude Code | `.claude/skills/linebind/` | A short block in `CLAUDE.md` |
| Antigravity | `.agents/skills/linebind/` | The same `AGENTS.md` block |

Setup copies the embedded skill offline and preserves existing instructions.
It does not create a contract, configure CI, install dependencies or approve
policy. `all` writes two identical skill copies without symlinks. Repeating the
same installation is a no-op, including under Git's LF/CRLF conversion.
Different existing content causes a conflict before writes; there is no force
overwrite. An operating-system write error can leave a partial installation.

Review and commit the generated files. Reload skills or start a new agent
session if discovery does not refresh. These repository-local locations follow
[Codex skills](https://learn.chatgpt.com/docs/build-skills),
[Claude Code skills](https://code.claude.com/docs/en/skills) and
[Antigravity skills](https://antigravity.google/docs/skills).
Claude receives its own instruction block so an existing `CLAUDE.md` does not
hide the guidance. See each provider's instructions:
[Codex](https://learn.chatgpt.com/docs/agent-configuration/agents-md),
[Claude](https://code.claude.com/docs/en/memory),
[Antigravity](https://antigravity.google/docs/rules/).

## Upgrade installed guidance

Upgrading the executable does not update previously committed skill files.
Generate the new bundle in a disposable repository:

```sh
GUIDANCE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/linebind-guidance.XXXXXX")
git -C "$GUIDANCE_DIR" init -q
linebind setup --repo "$GUIDANCE_DIR" --agent all
printf 'New guidance: %s\n' "$GUIDANCE_DIR"
```

Compare its skill directories and Linebind instruction blocks with your project.
Apply the reviewed changes to the targets you use, preserving custom guidance
and content outside those blocks. Commit the diff and reload skills.
`linebind setup --agent all --dry-run` reports unchanged files when the installed
content matches the bundle; deliberate customizations can still report conflicts.

## Use the skill

| Task | What to ask | Evidence to expect |
| --- | --- | --- |
| First contract | “Prepare a small contract from our documented decisions.” | Precise coverage and exclusions inside the contract; permitted and violating probes; explicit remaining gaps |
| Everyday change | “Implement this while respecting our contract.” | Application tests and the configured Linebind check; repair accidental violations without weakening policy |
| Decision change | “We have decided to adopt Zod; update the implementation and contract.” | A narrow policy diff, old-policy evidence and a separate proposed preview |

Existing files and dependencies are observations, not automatic decisions to
freeze. Subjective goals such as an elegant interface remain review criteria
unless a meaningful verifier exists. The skill identifies complementary tests,
builds and specialist checks instead of claiming that Linebind replaces them.

Your project designates the comparison revision. Resolve it to a full SHA and
keep it fixed for the task. A first preview needs an existing commit, but that
comparison commit need not contain a contract:

```sh
linebind validate
linebind preview --base "$BASE_SHA" --agent --html .linebind/preview.html
```

Follow the repository's CI mode for everyday work. For policy evolution, retain
the old-base check independently; an expected nonzero result must not prevent
the preview above from running:

```sh
linebind check --base "$BASE_SHA" --format json --output .linebind/check.json
```

`check` enforces the selected base contract and reports policy changes for
review. `preview` evaluates the proposed contract without adopting it. Approval
comes from the repository's review process; neither a passing preview nor an
agent-generated commit establishes approval. See [CI adoption](ci.md#bootstrap-and-policy-changes)
and [the trust model](trust-model.md#preview-a-proposed-contract).

An authorized decision change does not need the same permission twice. “Make
CI green” does not authorize relaxing a rule. Missing tools and incomplete
inputs require diagnosis, not an advisory rule or a different base.
External command checks need `--allow-exec`; candidate commands in preview may
be unreviewed, and their disposable copy is not a sandbox.

## Compatibility and boundaries

Installation tests verify file layout, preservation, conflicts and idempotence.
They cannot prove that an agent session loads or follows the skill. Local Codex
and Antigravity/Gemini trials exercised authoring and repair, but also found
missed cases and overbroad claims. The Claude trial stopped at authentication;
no successful Claude runtime trial is claimed. These are bounded experiments,
not a compatibility guarantee or independent production adoption.

The [canonical skill](../internal/agentsetup/skill/SKILL.md) and its references
are the maintained instructions. [CI](ci.md) runs independently of agent behavior.
Historical trial details remain in the
[pre-cleanup validation record](https://github.com/mchl-schrdng/linebind/blob/ffb15c8aeca8c5c9ee3987a4de8d18600aab17da/docs/validation.md).
