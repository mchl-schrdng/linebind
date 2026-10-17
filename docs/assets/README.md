# Artwork and evidence

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
