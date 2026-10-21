# The offline report

```sh
linebind check --base "$BASE_SHA" --format json --output .linebind/result.json --html .linebind/report.html
linebind report .linebind/result.json --output .linebind/saved-report.html
```

The first command writes the result of that check. The second renders previously
saved JSON without repeating analysis. Open HTML locally: it needs no server,
account or network resources. The display font and its license are embedded.

## Read the evidence

The contract line gives checks stable positions in declaration order. Filled
points pass; raised diamonds are required failures; lowered diamonds are advisory
failures; hollow question marks mean incomplete verification. Position does not
measure severity or time. Larger contracts use numbered marks and retain the
full check list.

Select a check to see its reason, scope, required/advisory setting, normalized
rule, findings and diagnostics. New, existing and resolved findings remain
distinct. **Evaluated rule** shows the policy actually used, with defaults and
empty operands preserved; it is not a full description of the verifier.

Policy changes appear separately, with before/after definitions matched by ID.
Renaming an ID appears as removal and addition. Changed command policy files
are attributed to their rules but must be reviewed separately. Formatting and
comments alone are not rule changes. An invalid side makes comparison unavailable.
The diff does not infer whether a change strengthens or weakens policy.

A contract-only change in `check` is labelled **Contract review required**.
Preview reports retain **PREVIEW / NOT APPROVED** in HTML, JSON and copied
text. Neither a preview nor a saved report approves a decision. See
[check and preview](trust-model.md#preview-a-proposed-contract).

The provenance section records revisions, input digest, contract hash and
capture time. These identify captured evidence; they are not an authentication
seal or live status. Review rule values, command arguments and diagnostics before
sharing reports containing private information.

## Navigate and share

The first required check needing attention opens by default, then advisory
issues if required checks passed. **Needs attention** filters to crossed and
unverified checks; **All checks** restores the full list. Direct links such as
`#check-1` reveal the selected check even when filtered out.

The global copy action includes all diagnostics. **Copy check** includes only
the selected check with provenance and global policy warnings; preview remains
labelled as unapproved. If clipboard access fails, the text is selected for
manual copying. Escape dismisses the selection.

Without JavaScript, every check and policy comparison remains readable. Printing
includes all checks and expands their details, then restores the interactive
state afterward.

## Format compatibility

New results use report schema 2 (introduced in v0.4.0). The renderer also reads
schema 1, marking missing recorded rule specifications. Older binaries reject
schema 2; render with a compatible version or share the standalone HTML.
The contract configuration remains version 1.

[Renderer and interaction tests](../CONTRIBUTING.md) cover evidence, escaping
and DOM behavior; they do not prove real-browser layout or accessibility.
[Artwork provenance](assets/README.md) distinguishes real fixture evidence
from decorative presentation.
