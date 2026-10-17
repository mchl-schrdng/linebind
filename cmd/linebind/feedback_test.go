package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
)

func TestCLIExtendedNativeRulesAndMergeBase(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	policy := `version: 1
name: Extended native rules
rules:
  - id: python
    kind: dependencies
    compare: new
    reason: Keep package names explicit.
    scope: [pyproject.toml]
    dependencies: {format: python, deny: [bad-package]}
  - id: cargo
    kind: dependencies
    reason: Keep crate names explicit.
    scope: [Cargo.toml]
    dependencies: {format: cargo, deny: [unsafe]}
  - id: environment
    kind: document
    reason: Deploy to known environments.
    scope: [config.json]
    document: {selector: /env, allowed-values: [staging, prod]}
  - id: version
    kind: document
    reason: Release versions must be explicit.
    scope: [config.json]
    document: {selector: /version, pattern: '^v[0-9]+$'}
`
	write(t, filepath.Join(dir, ".linebind.yml"), policy)
	write(t, filepath.Join(dir, "pyproject.toml"), "[project]\ndependencies=['Bad_Package']\n")
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\n[dependencies]\nsafe='1'\n")
	write(t, filepath.Join(dir, "config.json"), `{"env":"prod","version":"v1"}`)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "base")
	git(t, dir, "branch", "approved")
	cli(t, 0, "validate", "--repo", dir)
	for _, mode := range []string{"check", "preview"} {
		output := cli(t, 0, mode, "--repo", dir, "--base", "merge-base:approved", "--format", "json")
		var report model.Report
		if err := json.Unmarshal([]byte(output), &report); err != nil {
			t.Fatal(err)
		}
		if len(report.Base) != 40 || len(report.Checks) != 4 || report.Checks[0].Findings[0].Evolution != "existing" {
			t.Fatal(report)
		}
	}
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\n[dependencies]\nsafe={package='unsafe',version='1'}\n")
	write(t, filepath.Join(dir, "config.json"), `{"env":"dev","version":3}`)
	cli(t, 1, "check", "--repo", dir, "--base", "merge-base:approved", "--agent")
	write(t, filepath.Join(dir, "Cargo.toml"), "[package]\n[dependencies]\nsafe={path='../unsafe'}\n")
	write(t, filepath.Join(dir, "config.json"), `{"env":"prod","version":"v1"}`)
	cli(t, 2, "check", "--repo", dir, "--base", "merge-base:approved", "--agent")
}
