# Conftest Kubernetes example

This example applies a real Rego policy to Kubernetes v1 Pods in `infra/`,
including `.yaml` and `.yml` files and every document in a YAML stream. Regular,
init and ephemeral containers must omit `securityContext.privileged` or set it
to boolean `false`. A `true`, string or null value crosses the rule. Omitting
the field is permitted because privileged mode defaults to false.

Unsupported resource kinds, missing or empty `spec.containers`, malformed
container groups and malformed security contexts cross this example's rule;
they cannot silently bypass the Pod check. The policy validates only this
property and the shapes needed to inspect it. It is not a complete Kubernetes
schema, Pod Security Standards implementation or live-cluster verifier.
Deployment templates, Helm output, Windows host processes and resources outside
`infra/` need their own explicit policy and scope. It does not establish that
containers run as non-root.

Validated with [Conftest **0.62.0**](https://github.com/open-policy-agent/conftest/releases/tag/v0.62.0)
(OPA **1.6.0**) on macOS arm64. Install the pinned official release, verify its
published checksum, and place `conftest` on `PATH`. The runner does not download
tools or verify their version at runtime.

```sh
# Run from this example directory, with linebind and conftest on PATH.
demo_dir=$(mktemp -d)
cp -R base/. "$demo_dir/"
git -C "$demo_dir" init
git -C "$demo_dir" add .
git -C "$demo_dir" commit -m "Approve workload policy contract"
linebind check --repo "$demo_dir" --base HEAD --allow-exec

cp -R crossed/. "$demo_dir/"
linebind check --repo "$demo_dir" --base HEAD --allow-exec
```

The first check exits **0**. The second exits **1** and names the privileged
container. Use `unverified/` instead to get exit **2** from malformed YAML. Reset
source files to `base/` between scenarios. `--allow-exec` explicitly trusts
commands on this host; the disposable copy is not a sandbox.

The command is `conftest test infra --policy policy --output json`.
`policy-files: [policy/main.rego]` supplies the approved policy to the disposable
copy. Candidate policy edits are also reported and block the gate.

Each file in the declared scope must have at least one evaluated policy result.
Zero evaluated rules, missing files, skipped checks and exceptions produce an
unverified result. Failures and warnings both cross the Linebind rule. Combined
or stdin results without an individual scoped filename are unsupported.

Official result structure: [Conftest 0.62.0 output](https://github.com/open-policy-agent/conftest/blob/v0.62.0/output/result.go).

The mutation panel in `internal/app/policy_panel_test.go` exercises normal
changes, all three container groups, malformed shapes, unsupported resource
kinds and YAML streams against this exact policy. Kubernetes documents the
three relevant fields in its [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).
