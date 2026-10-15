# Working on Linebind

Read the project's [Linebind skill](internal/agentsetup/skill/SKILL.md) when
creating, checking or evolving this repository's contract. This source repository
uses the bundled skill directly; keep its canonical content in that directory.

The maintained `origin/main` branch supplies the approved policy for local work.
Resolve it to a full commit SHA at the start of a task and keep that SHA fixed.
CI uses the explicit pull-request base and head SHAs instead. Do not substitute
the candidate's own revision to make a check pass.

Use a trusted released `linebind` executable (`./bin/linebind` when available).
Run `check --base <fixed-sha> --agent` before finishing changes. For a requested
policy evolution, retain the old-base check and a separate `preview`; report
the expected policy-review result. See [our contract](docs/self-contract.md)
for scope and adoption. Run the relevant Go tests separately.
