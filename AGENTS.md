# Working on Linebind

Read the project's [Linebind skill](internal/agentsetup/skill/SKILL.md) when
creating, checking or evolving this repository's contract. This source repository
uses the bundled skill directly; keep its canonical content in that directory.

Resolve `origin/main` to a full commit SHA at the start of local work and keep
that comparison fixed. CI uses explicit pull-request base and head SHAs.
Never substitute the candidate as its own base just to obtain a pass.

Use a trusted released `linebind` executable (`./bin/linebind` when available).
Our CI uses `preview` to validate the proposed contract; contract edits alone
must not fail it. Run the same validation locally. For policy evolution, keep
an old-base `check` as separate evidence; its policy-review exit is expected.
The maintainer adopts the proposal by merging after all required CI checks pass.

Never merge with failed or pending required CI, use an administrative bypass,
or disable protections to finish a task. No second reviewer is required.
Run relevant Go tests separately. See [our contract](docs/self-contract.md)
and [CI integration](docs/ci.md) for scope and adoption.
