# Dependency-cruiser boundary example

This example checks a real import relation: client code may use `src/api`, but may
not import `src/domain` directly. No project dependencies are installed or loaded.
The approved `rules.cjs` supplies the graph policy. A separate approved Node
script checks JavaScript syntax for every `.mjs` file in the declared source tree.

Validated with dependency-cruiser **17.3.8** and Node **25.9.0** on macOS arm64.
Install that dependency-cruiser version in a separate tools directory, then put
its `node_modules/.bin` on `PATH`. The runner does not download tools or verify
their version at runtime.

```sh
tools_dir=$(mktemp -d)
npm install --prefix "$tools_dir" --ignore-scripts --no-audit --no-fund dependency-cruiser@17.3.8
export PATH="$tools_dir/node_modules/.bin:$PATH"

# Run from this example directory, with linebind and Node on PATH.
demo_dir=$(mktemp -d)
cp -R base/. "$demo_dir/"
git -C "$demo_dir" init
git -C "$demo_dir" add .
git -C "$demo_dir" commit -m "Approve import boundary contract"
linebind check --repo "$demo_dir" --base HEAD --allow-exec

cp -R crossed/. "$demo_dir/"
linebind check --repo "$demo_dir" --base HEAD --allow-exec
```

The first check exits **0**. The second exits **1** and identifies the forbidden
client-to-domain edge. Use `unverified/` instead to get exit **2** from an unresolved
import. Use `invalid-syntax/` to get exit **1** from the syntax check. Reset source
files to `base/` between scenarios. `--allow-exec` explicitly trusts commands to
run on this host; the disposable copy is not a sandbox.

The two commands are `node scripts/check-syntax.mjs` and
`depcruise --config rules.cjs --output-type json src`. `policy-files` ensures the
approved versions of both policy files are used. Candidate policy edits are also
reported and block the gate.

Dependency-cruiser [falls back to a permissive parser on syntax errors](https://github.com/sverweij/dependency-cruiser/blob/v17.3.8/src/extract/acorn/parse.mjs).
Its JSON does not report that fallback: passing this adapter means its reported
graph satisfies the configured rules, not that the source syntax is valid. This
is why the example includes a separate required syntax check. For TypeScript,
use a separately configured compiler check such as `tsc --noEmit`.

Linebind checks that all scoped files appear in the reported graph. Unresolved
imports, missing module results, graph filters, consolidated nodes and built-in
tool baselines are unverified. Warning and information violations also cross the
Linebind rule. Keep the declared scope limited to files the tool actually analyzes.

Official format reference: [dependency-cruiser JSON output](https://github.com/sverweij/dependency-cruiser/blob/v17.3.8/doc/output-format.md).
