---
name: linebind
description: Create, check, and evolve a Linebind repository contract from explicit project decisions. Use when adopting Linebind, implementing work in a repository governed by its contract, investigating a Linebind result, or proposing a policy change. Does not replace general architecture review or invent project decisions.
---

# Linebind

Use the installed CLI to keep project decisions verifiable across coding agents.
The skill helps make decisions; the CLI evaluates declared checks. Neither a
passing result nor a well-written contract establishes whole-repository quality.

## Establish the context

- Read the repository's agent instructions, the relevant decisions, and its
  contract (normally `.linebind.yml`). Preserve existing work and authorization.
- Use `linebind` on `PATH` or the executable path supplied by the user. Read
  [the operating reference](references/operations.md) for prerequisites,
  commands, result interpretation, or a missing executable.
- Read which CI mode the repository has chosen: `check` enforces adopted rules;
  `preview` validates proposed rules before maintainer adoption. Follow that
  workflow without treating a passing proposal as approval.
- Obtain the agreed base from the task or repository's documented workflow. Resolve it to a full commit SHA and keep that SHA fixed
  during the task. A branch name alone does not establish approval. Do not use
  the candidate as its own approved base to make a result pass.
- If an approved-base check has no designated base, prepare independent work
  and ask only for that missing decision. Do not silently substitute `HEAD` or
  `main`. A draft preview may use an existing commit as a comparison baseline,
  explicitly identified as comparison rather than approval.

## Choose the relevant workflow

### Adopt a first contract

Read [the authoring reference](references/authoring.md). Extract a few decisions
that the user or project has actually established. Existing files and packages
are evidence of implementation, not automatic decisions to freeze forever.
Keep intentions without a meaningful verifier as review criteria.

Create the contract, explain each rule's reason and coverage, then validate
and preview it. Exercise a representative violation in a disposable fixture
without damaging the user's working tree. Report the observed detection and
the scope still unchecked. Adoption into the trusted branch follows the
repository's review process; neither `init`, `validate`, nor `preview` approves
the contract. Do not create a commit merely to manufacture approval.

### Develop under an existing contract

Implement the requested change and run the repository's configured validation.
Use `check` against the fixed approved base when adopted-policy evidence is needed.
Read the checks, diagnostics, and policy changes, not just the exit code. Correct
accidental violations in the code when the task's intended behavior can be
preserved. Continue ordinary compliant work without requesting permission.

Do not remove a rule, widen its scope exceptions, make it advisory, or choose
a different base simply to obtain a passing result. Missing tools, missing
inputs, or incomplete verification need an operational diagnosis, not a weaker
contract. Report unfinished verification precisely while completing independent
work that remains possible.

### Evolve a decision

If the user has already authorized the decision change, prepare the code and
contract change within that scope; do not ask for the same authorization again.
If the intent is ambiguous, first prepare the concrete alternative and its
consequences, then ask one focused question when an actual tradeoff needs the
user. A generic request to fix CI is not a decision to relax a boundary.

Explain which rule changes, why, and what protection is added or removed. Keep
two distinct pieces of evidence: `check` against the original approved base,
and `preview` of the proposed contract against the same comparison base. The
preview may pass while the old-policy check reports the change. If CI uses
`preview`, a policy edit alone must not fail it; the maintainer adopts the
proposal through the documented merge process after required CI passes. If CI
uses `check`, retain its stricter adoption process. Do not claim that
conversation authorization, a successful preview, or a PR label bypasses the
repository's approval process. Adopt a new trusted base only after that process
has actually completed.

## Finish with evidence

State what changed, the base and candidate checked, the observed result, and
any unresolved verification or policy review. Re-run the applicable command
after relevant edits; a saved report describes its captured inputs, not later
changes. Mention only checks the tool actually performed. Linebind needs no
LLM API key; external command verifiers may have their own prerequisites.
