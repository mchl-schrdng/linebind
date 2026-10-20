# Product visuals and evidence

## Official identity

Linebind's signature colors are midnight blue `#0d1024` and coral orange
`#d65c46`. Use dark ink `#171830` for text on light surfaces, dark coral
`#b94232` for readable accents on white, and light coral `#ed8c75` on navy.
The CLI's HTML report keeps a light reading surface and distinct green, coral
and amber statuses. Brand colors never stand in for a passing check.

The report embeds Syne for headings and the wordmark; evidence uses system
sans-serif and monospace fonts. Its Latin variable WOFF2 is the same asset as
Cloud's, downloaded from [Fontsource](https://cdn.jsdelivr.net/fontsource/fonts/syne:vf@latest/latin-wght-normal.woff2)
on 20 October 2026. The [SIL Open Font License](../../internal/report/fonts/OFL.txt)
is bundled in every generated report. Reports make no font or image request.

The README's [mecha](mecha-line.webp) is original decorative artwork, generated
with the built-in Imagegen tool on 20 October 2026 and shared with Linebind
Cloud. The WebP is 1122 × 1402 pixels. Its ivory machine holds a single coral
line against midnight blue; it illustrates the product's boundary motif.
It is not a product screenshot or evidence of a check. The approved direction
uses polished manga and mecha illustration, with an Evangelion influence but
no existing character or unit. The source generation remains in the owner's
image library. [Full generation prompt](mecha-prompt.md)

The README keeps the measured v0.4.0 screenshots below as historical examples.
Their original appearance and recorded evidence are unchanged; the current
source template uses the new identity and will ship in the next CLI release.

## README visuals — v0.4.0

The README presents real Linebind evidence in four **Mac-style compositions**.
The original desktop wallpaper and window chrome are decorative artwork. The
report content comes from unmodified browser captures; the Terminal view
renders captured stdout with only colors and visual line wrapping added. These
are editorial presentations, not photographs or operating-system screenshots.

All scenarios are disposable `react-storefront` fixtures, not production
applications. No packages were installed and no project code was executed.

| Visual | Content and retained evidence |
| --- | --- |
| [Contract line](report-v0.4.0/mac/line-mac.png) | Close-up of the crossed report: one required dependency rule fails and four checks pass. [JSON](report-v0.4.0/report-crossed.json) · [HTML](report-v0.4.0/report-crossed.html) |
| [Complete interface](report-v0.4.0/mac/report-mac.png) | The same report from its header through the check list, selected evidence, provenance disclosure and footer. |
| [Terminal](report-v0.4.0/mac/terminal-mac.png) | The README's one-rule contract rejects jQuery; exit `1`. [Raw stdout](report-v0.4.0/mac/cli.txt) · [Command](report-v0.4.0/mac/cli-command.txt) · [JSON](report-v0.4.0/mac/cli.json) · [HTML](report-v0.4.0/mac/cli.html) |
| [Policy preview](report-v0.4.0/mac/preview-mac.png) | Zod is proposed in the manifest and allowlist. All five checks pass; exit `0`. The preview notice and full before/after definitions remain visible. [JSON](report-v0.4.0/report-preview.json) · [HTML](report-v0.4.0/report-preview.html) |

The Zod candidate was also checked against the unchanged original policy:
[the old-policy result](report-v0.4.0/proposal-check.json) returns `1`. A passing
preview does not adopt the proposal.

### Evidence provenance

- CLI source: [`083cbdc5bdec9ba8a1173e9524878db8a8237beb`](https://github.com/mchl-schrdng/linebind/commit/083cbdc5bdec9ba8a1173e9524878db8a8237beb), the released v0.4.0 revision.
- Archive: [`linebind_0.4.0_darwin_arm64.tar.gz`](https://github.com/mchl-schrdng/linebind/releases/tag/v0.4.0), verified against the release's `SHA256SUMS`.
- Archive SHA-256: `c3b5c4c41f3100f376ab74ecdb6fe71d94524288b10b4c9372728fc0e6d1643d`.
- Five-rule fixture base: `1aecd55ba3edce19abd7b065d42b2561ccfca6f3`.
- One-rule CLI fixture base: `132a20336b59b3a102782bfea5a6873565d0e50c`.
- Reports generated on **17 October 2026**: five-rule reports at **06:10 UTC**, CLI evidence at **07:05 UTC**. Exact timestamps and contract hashes remain in the evidence files.

The fixture commits are synthetic comparison points, not Linebind revisions or
proof of real project approval. The CLI's `main` refers to its temporary fixture.

| Scenario | Recorded input digest (SHA-256) |
| --- | --- |
| Five-rule jQuery check | `fced46cafc7ee21494b800be514be54bb5223bca09abd28e1747af0cfd217931` |
| Zod check and preview | `62f9589572c3bfeb21cd9c80e4bb8571042a11624a67dc22262d442690730164` |
| One-rule CLI check | `8229618d0fef461a2bdb52ea3fa3c4ec68d40066f132f54a72cedb3b430ba50b` |

### Capture and composition

The browser was **Codex In-app Browser on macOS**, with its default **1280 ×
720 CSS-pixel viewport** (1265-pixel document width, excluding the scrollbar).
Native viewport exports are **1265 × 712 pixels**. The original viewport images
are retained beside the compositions:

- [Report top](report-v0.4.0/mac/report-top.jpg), scroll offset `0`, and [report bottom](report-v0.4.0/mac/report-bottom.jpg), offset `599` CSS pixels.
- [Preview top](report-v0.4.0/mac/preview-top.jpg), offset `0`, and [preview bottom](report-v0.4.0/mac/preview-bottom.jpg), offset `720` CSS pixels, with the modified rule expanded.

The renderer joins the overlapping viewport captures at their measured scroll
positions, removes the scrollbar strip and adds the decorative frame. It checks
the shared image strip for alignment. The line detail is a crop of the original
report header and line; the preview stops just below its complete policy diff.
No report text or CSS is rewritten. The CLI prompt, title and exit footer are
presentation labels outside the preserved stdout.

The [wallpaper](report-v0.4.0/mac/wallpaper.png) is original AI-generated artwork.
The four compositions are 1600 pixels wide: line **758 px** high, complete
report **1699 px**, Terminal **888 px**, preview **1740 px**. Browser content
keeps its original aspect ratio. Earlier 632-pixel report exports are retained
in this directory but are no longer used in the README.

### Reproduce

From the repository root, pass an absolute path to the trusted released binary
and an absolute output directory:

```sh
bash docs/assets/report-v0.4.0/reproduce.sh /absolute/path/to/linebind /tmp/linebind-readme-reports
bash docs/assets/report-v0.4.0/mac/capture-cli.sh /absolute/path/to/linebind /tmp/linebind-cli-evidence
```

Both scripts include every fixture input, verify the conforming baseline and
assert the expected failing exits. Each fixture is temporary and removed after
capture. Fresh timestamps and durations will differ. Open the generated reports
in a browser; expand the modified rule for the preview view.

To rebuild the compositions from their retained viewport captures and stdout,
use Node.js with `sharp` available and run:

```sh
node docs/assets/report-v0.4.0/mac/compose.cjs
```

The renderer builds SVG layouts in memory and exports PNGs. Node.js and `sharp`
are artwork-only tools; neither is required by Linebind or its Go build.

## Earlier artwork

These retained assets document earlier presentations. They are not the current
README's screenshots.

The Linebind symbol and wordmark are original vector artwork, supplied in light
and dark variants under this repository's MIT license. The brackets represent
an explicit boundary; the line ties the two sides together.

`contract-line.svg` is a static illustration, not a report capture or a live
status badge. Its five positions refer to the repository's own contract rules,
in declaration order; labels are shortened for the composition. The middle row
illustrates a hypothetical `exec.Command` occurrence in the native contract
package, which its declared lexical rule forbids. The final row illustrates its
removal. No execution or measured finding is asserted by this artwork. The SVG
includes light/dark palettes and contains no scripts or external resources.

`report-desktop.svg` is an editorial illustration of the report on a
macOS-style desktop, not a browser or operating-system screenshot. The window
layout is adapted from the actual report template. Its finding, rule, reason,
scope, counts and remediation come from a real CLI run of the included
`examples/frontend` fixture with its `crossed` mutation. No user desktop or
private project was captured. The original JSON evidence is
`frontend-crossed.json`; its development version and timestamp are retained.

The illustrated rule is strict, so its finding correctly reads “Not comparable”.
The candidate reference is shortened to “working tree” for the composition;
the JSON keeps its full identity. The PNG is a raster export of this SVG.

Reproduce the source scenario with the installed CLI:

```sh
go run ./scripts/pilots -binary /absolute/path/to/linebind
```

This produces fresh JSON and actual standalone HTML reports. The illustration
does not establish browser rendering or interaction compatibility. DOM-based
tests and native report tests are separate from visual browser QA.

## CLI output

`frontend-cli.png` is a faithful rendering of stdout captured from the published
Linebind v0.1.0 macOS ARM64 binary. It is not a screenshot of the Terminal app.
Only visual line wrapping and text colors were added; no diagnostic was removed
or rewritten. `frontend-cli.svg` is the image source and `frontend-cli.txt` is
the raw stdout. The shell prompt, window heading and exit-code footer are
presentation labels outside that stdout.

The run used a fresh `react-storefront` Git repository containing
`examples/frontend/base/.linebind.yml`, the base `package.json`, and an ignore
entry for `/.linebind/`. Those files were committed on `main`; on
`feature/product-carousel`, the manifest was replaced by
`examples/frontend/crossed/package.json`. The command was:

```sh
linebind check --base main --html .linebind/report.html
```

It returned exit code `1`. A second check with `--format json --output
.linebind/report.json` against the same captured inputs produced the preserved
`frontend-cli.json`; its hashes identify the base, contract and candidate input.
The scenario tests a seeded manifest change, not a built carousel or an actual
production application. The native check needs neither npm installation nor a
model API key.
