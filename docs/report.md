# The offline report

`linebind check --base REF --html report.html` writes the result of that run.
`linebind report result.json --output report.html` renders a saved JSON result
without repeating the analysis. Open the resulting HTML file locally; it needs
no server, account or network resource. The display font and its SIL Open Font
License are embedded in the HTML; repository content still uses a system font.

The contract line gives each check a stable position in declaration order.
A filled point is passing, a raised diamond is a required failure, a lowered
diamond is an advisory failure, and a hollow question mark is unverified.
The displacement is categorical: it measures neither severity nor time.
Selecting a mark opens the same clause as the check navigation, including checks
hidden by the attention filter. Larger contracts use numbered marks; every
mark keeps its accessible name and the full check list remains available.
Contract changes are labelled separately and do not recolor the reference line.

**Evaluated rule** shows normalized YAML from the policy actually used, with
defaults included. It preserves operands such as `allow: []` and `equals: null`.
This describes the declared verifier, not its entire implementation or tool
version. Before sharing reports, review rule values and command arguments as
well as diagnostics for sensitive data.

When policy changes, the report compares rules by ID and shows their base and
candidate definitions. Renaming an ID appears as removal and addition. Changed
declared command policy files are attributed to the rules that reference them,
even if their YAML is unchanged; review those files separately. Formatting and
comments alone do not become rule changes. An invalid side makes rule comparison
unavailable rather than implying no changes. These records describe edits,
not inferred strengthening or weakening, and do not change CI outcomes.

Starting with v0.4.0, new results use report schema **2**. This version can also render schema 1
reports, with an explicit notice when a rule specification was not recorded.
Older binaries reject schema 2 JSON; regenerate HTML with this version or share
the standalone HTML. The contract configuration remains version 1.

`linebind preview --base REF --html preview.html` produces an explicitly
labelled **PREVIEW / NOT APPROVED** report using the candidate's proposed policy.
Its JSON has `"purpose": "preview"`, and `linebind report` preserves that label
when rendering it later. An earlier v0.1.0 binary rejects preview JSON as an
unknown format field. A repository can use preview to validate its proposed
contract in CI; a passing result does not approve that contract or establish
compliance with the base policy. The maintainer reviews the contract diff and
adopts it through the repository's merge process.
See the [agent workflow](agents.md#what-preview-establishes).

Each check retains its reason, required or advisory setting, verification
method, declared scope, findings and diagnostics. New, existing and resolved
findings remain distinct. Contract changes appear separately from check
results. A short explanation distinguishes blocking required results from
advisory results; incomplete verification stays visible. The capture time is
shown near the summary so a saved report is not mistaken for live status.
In an approved-base `check`, a contract-only change is titled **Contract review
required**, rather than suggesting that a rule was violated. Coexisting
violations or incomplete checks remain visible in the headline and evidence.
The provenance section records the revisions, input digest and
contract hash; these are traceability information, not an authentication seal.

With JavaScript enabled, the first required check needing attention opens by
default, followed by advisory issues if all required checks passed. Select any
check to read its details. **Needs attention** filters the navigation to crossed
and unverified checks, including advisory results. **All checks** restores the
full list. Direct links such as `#check-1` take precedence and reveal their check
even when it was filtered out. Without JavaScript, every check remains readable
in the page, with rule definitions and policy comparisons expanded. Printing
includes every check and expands these details too; afterward, the interactive
report restores the reader's open and closed sections.

The copy action always copies the complete plain-text diagnostic, including
checks hidden by the filter. If clipboard access is unavailable, it selects
the text for manual copying.
Press Escape to close that selection.

**Copy check** copies only the selected check's definition, findings and
diagnostics, with the report's comparison, hashes, capture time and policy source.
It retains global policy warnings and identifies the header status as the
overall report outcome. Preview excerpts remain explicitly unapproved. The
same clipboard fallback and Escape behavior apply to individual checks.

## Interaction checks

The Go renderer tests check escaping, evidence preservation and text output.
An optional DOM test generates a real report through the CLI, then checks its
script against an in-memory DOM with resource and network loading disabled.
It covers initial issue selection, filtering, hash changes, clipboard success
and failure, keyboard dismissal, focus restoration and content without
JavaScript. Renderer tests also distinguish required, advisory and unknown
outcomes without treating incomplete checks as passing.

With Node.js and npm available, these POSIX shell commands keep the test-only
dependency outside the checkout:

```sh
npm install --prefix /tmp/linebind-ui-test --no-save --ignore-scripts --no-audit --no-fund jsdom@26.1.0
go build -o ./bin/linebind ./cmd/linebind
NODE_PATH=/tmp/linebind-ui-test/node_modules node scripts/ui-test.mjs ./bin/linebind
```

These tests do not establish visual layout, real-browser clipboard permissions
or assistive-technology behavior. Browser and accessibility review remain
separate checks.

## Artwork and report images

The Linebind symbol and outlined wordmark in `docs/assets/` are original SVG
artwork shipped under the repository license. They use no external font or
third-party image. The report uses embedded Syne for its heading and wordmark,
and system fonts for evidence. Its midnight-blue and coral frame matches
Cloud; green, coral and amber still distinguish the check statuses.
[Brand colors and artwork provenance](assets/README.md#official-identity)

When adding a report image, capture an actual CLI-generated report and retain
its JSON/HTML, invocation, source revision and input digest with the capture
notes. Record the browser and viewport used. Identify a seeded example as a
fixture; do not present its findings as findings from a production repository.
An illustration or design study should be labeled as such, rather than as a
report screenshot. Redact sensitive inputs before generating shareable reports.
