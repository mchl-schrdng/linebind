# Product visuals and evidence

## Official identity

The original SVG logos and wordmarks use the repository's MIT license. The
signature colors are midnight blue `#0d1024` and coral `#d65c46`; use dark coral
`#b94232` on white and light coral `#ed8c75` on navy. Status colors retain their
own meanings. The standalone marks are available for project avatars and
sharing: [light background](logo.svg) and [dark background](logo-dark.svg).

The report embeds Syne from Fontsource with its
[SIL Open Font License](../../internal/report/fonts/OFL.txt); evidence uses
system fonts. No runtime font or image request is made.

## README captures

These two images show real **v0.4.0** report content inside decorative Mac-style
frames. They are historical fixture captures, not production findings or
screenshots of the current release. The wallpaper and window chrome are artwork.

| Image | Retained evidence |
| --- | --- |
| [Contract line](report-v0.4.0/mac/line-mac.png) | [JSON](report-v0.4.0/report-crossed.json): jQuery crosses one required rule; four pass |
| [Policy preview](report-v0.4.0/mac/preview-mac.png) | [JSON](report-v0.4.0/report-preview.json): proposed Zod allowlist passes; [old-policy check](report-v0.4.0/proposal-check.json) still exits `1` |

The disposable `react-storefront` fixture used the checksum-verified
[v0.4.0 macOS ARM64 release](https://github.com/mchl-schrdng/linebind/releases/tag/v0.4.0),
source `083cbdc5bdec9ba8a1173e9524878db8a8237beb`, on 17 October 2026.
The archive SHA-256 was `c3b5c4c41f3100f376ab74ecdb6fe71d94524288b10b4c9372728fc0e6d1643d`.
Browser captures used Codex In-app Browser at 1280 × 720 CSS pixels. JSON retains
exact input digests, policy hashes and timestamps. No project packages or code ran.

The fixture inputs and expected exits remain reproducible:

```sh
bash docs/assets/report-v0.4.0/reproduce.sh /absolute/path/to/trusted-linebind /tmp/linebind-readme-reports
```

Use the historical v0.4.0 binary to reproduce that version. Fresh capture times
will differ. Saved JSON can be rendered with `linebind report`; generated HTML
need not be duplicated in Git. Original composition sources and unused older
artwork remain in
[Git history](https://github.com/mchl-schrdng/linebind/tree/ffb15c8aeca8c5c9ee3987a4de8d18600aab17da/docs/assets).

For new captures, retain the input recipe, JSON, invocation, binary revision and
browser/viewport details. Label seeded examples and decorative artwork honestly.
