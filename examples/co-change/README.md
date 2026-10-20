# A migration and its delivery notes

Requires Linebind v0.5.0. The contract says that changing any file in
`migrations/**` requires adding or changing `docs/migrations.md` in the same
base-to-candidate comparison.

`scripts/pilots` runs this fixture through the installed binary and reproduces
each HTML report from saved JSON:

| Variant | Expected result |
| --- | --- |
| `base` unchanged | Passed, exit 0 |
| `crossed` changes SQL without notes | Crossed, exit 1 |
| `unverified` replaces the notes with an unavailable Git LFS pointer | Unverified, exit 2 |
| Contract removed with the SQL change | Crossed under the base contract, exit 1 |
| `repaired` changes both SQL and notes | Passed, exit 0 |

This is a synthetic example, not a defect discovered in another project.
Linebind verifies the accompanying edit, not SQL correctness or the quality
of the notes. Deleting the notes does not satisfy the obligation. Whitespace
edits do, so ordinary review remains necessary.
