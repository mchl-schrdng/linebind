# Security

Please report vulnerabilities through GitHub's
[private vulnerability reporting](https://github.com/mchl-schrdng/linebind/security/advisories/new).
Include the affected version, platform, minimal reproduction and expected trust
boundary. Do not include credentials or private repository content.

If private reporting is unavailable, open an issue asking for a private contact
channel without publishing the vulnerability details. Security fixes target the
latest released version; older releases do not have a separate maintenance branch.
There is no guaranteed response time.

## Trust boundary

Linebind checks declared repository rules. A passing result is not a security
audit or proof that arbitrary code is safe. Review the trusted base revision,
contract, referenced tools and the workflow that invokes the verifier.

Command checks run programs with the invoking user's permissions when explicitly
enabled. Linebind is not an operating-system sandbox. Use an isolated runner with
restricted credentials when executing commands from an untrusted repository.
Treat report evidence and tool output as untrusted content.

Release archives include SHA-256 checksums to detect corruption. Checksums served
beside an archive do not independently authenticate the publisher. Download both
from this repository's GitHub releases. Releases do not provide signed release
attestations, macOS notarization or Windows Authenticode signatures.
