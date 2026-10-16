# Dependency-cruiser architecture example

This example makes a small system design executable. The approved `rules.cjs`
declares the allowed dependencies between five layers; all other edges and all
cycles are forbidden. No project dependencies are installed or loaded.

| Source layer | May import |
| --- | --- |
| `client` | `client`, `api`, `shared` |
| `api` | `api`, `domain`, `adapters`, `shared` |
| `domain` | `domain`, `shared` |
| `adapters` | `adapters`, `domain`, `shared`, Node built-ins |
| `shared` | `shared` |

Adding modules inside a layer is allowed. A new, unmapped layer is rejected even
when its module has no imports or consumers. Moving a forbidden domain import
into a shared relay or re-export does not make it acceptable: the shared-to-domain
edge violates the map. An API re-export remains allowed.

The `src` tree in this example contains only JavaScript modules (`.js`, `.mjs`,
`.cjs`). Its scope is `src/**`, so a new extension cannot silently leave the
declared inputs. A separate approved Node script checks syntax; an unsupported
format exits **2**, leaving verification incomplete. For a mixed frontend tree,
define which files are executable source and configure the appropriate parsers
and compiler checks before adopting this recipe.

This dependency-free example is not a drop-in React, TypeScript or npm workspace
integration. Application packages are not provisioned in the disposable snapshot,
and ignored `node_modules` are not copied. The current structured adapter requires
captured module coverage and treats `doNotFollow` as incomplete; skipping external
packages is not a validated workaround. React imports, TSX with path aliases and
local workspace imports all returned `unverified` in adoption probes. A tested
package-resolution and execution strategy is still needed for those projects.

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
base=$(git -C "$demo_dir" rev-parse HEAD)
linebind check --repo "$demo_dir" --base "$base" --allow-exec

cp -R crossed/. "$demo_dir/"
linebind check --repo "$demo_dir" --base "$base" --allow-exec
```

The first check exits **0**. The second exits **1** and identifies the forbidden
client-to-domain edge. Use `unverified/` instead to get exit **2** from an unresolved
import. Use `invalid-syntax/` to get exit **1** from the syntax check. Reset source
files to `base/` between scenarios. `--allow-exec` explicitly trusts commands to
run on this host; the disposable copy is not a sandbox.

The two commands are `node scripts/check-syntax.mjs` and
`depcruise --config rules.cjs --output-type json src`. `policy-files` ensures the
approved versions of both policy files are used. Candidate policy edits are also
reported and block this example's `check` gate. To evaluate an intended map change,
run `linebind preview` against the same base; merging is the adoption step.

Dependency-cruiser [falls back to a permissive parser on syntax errors](https://github.com/sverweij/dependency-cruiser/blob/v17.3.8/src/extract/acorn/parse.mjs).
Its JSON does not report that fallback: passing this adapter means its reported
graph satisfies the configured rules, not that the source syntax is valid. This
is why the example includes a separate required syntax check. For TypeScript,
use a separately configured compiler check such as `tsc --noEmit`.

Linebind checks that all scoped files appear in the reported graph. Unresolved
imports, missing module results, graph filters, consolidated nodes and built-in
tool baselines are unverified. Warning and information violations also cross the
Linebind rule.

Literal dynamic imports such as `import('../domain/profile.mjs')` are checked.
Computed imports such as `import(target)` can be omitted by dependency-cruiser
and **can pass despite violating the intended boundary**. The map also does not
verify business behavior, HTTP calls, runtime loading, or whether a module belongs
in the layer its path names. These require separate checks or review.

The repeatable [architecture panel](../../internal/app/architecture_panel_test.go)
exercises legitimate refactors, direct and indirect violations, cycles, unclassified
modules, incomplete inputs and the known computed-import gap. These controlled
cases establish this recipe's behavior, not a general architecture detection rate.

Official format reference: [dependency-cruiser JSON output](https://github.com/sverweij/dependency-cruiser/blob/v17.3.8/doc/output-format.md).
