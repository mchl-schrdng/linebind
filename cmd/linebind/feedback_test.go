package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestFeedbackSeparatesCoexistingCausesWithoutChangingExitCodes(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	policy := `version: 1
name: Feedback fixture
rules:
  - id: readme
    kind: require-files
    reason: Keep the entry point.
    files: {require: [README.md]}
  - id: marker
    kind: forbid-text
    reason: Keep the literal marker out of shipped text.
    scope: [source.txt]
    text: {pattern: DEBUG_ONLY}
  - id: config
    kind: document
    reason: Preserve the declared setting.
    scope: [config.json]
    document: {selector: /enabled, equals: true}
`
	write(t, filepath.Join(dir, ".linebind.yml"), policy)
	write(t, filepath.Join(dir, "README.md"), "Fixture\n")
	write(t, filepath.Join(dir, "source.txt"), "Allowed\n")
	write(t, filepath.Join(dir, "config.json"), `{"enabled":true}`)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "baseline")
	baseBytes, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseBytes))
	for _, test := range []struct {
		name, mode, source, config string
		policyChange, advisory     bool
		exit                       int
		want, absent               []string
	}{
		{name: "passed", exit: 0, want: []string{"required checks: 3 passed, 0 crossed, 0 unverified", "Re-run after relevant edits"}, absent: []string{"Repair unintended", "Resolve incomplete", "Review policy changes"}},
		{name: "policy only", policyChange: true, exit: 1, want: []string{"required checks: 3 passed, 0 crossed, 0 unverified", "policy files changed: 1", "Review policy changes separately", "same base"}, absent: []string{"Repair unintended", "Resolve incomplete"}},
		{name: "violation", source: "DEBUG_ONLY", exit: 1, want: []string{"required checks: 2 passed, 1 crossed, 0 unverified", "Repair unintended violations"}, absent: []string{"Resolve incomplete", "Review policy changes"}},
		{name: "incomplete", config: "{", exit: 2, want: []string{"required checks: 2 passed, 0 crossed, 1 unverified", "Resolve incomplete verification"}, absent: []string{"Repair unintended", "Review policy changes"}},
		{name: "mixed", source: "DEBUG_ONLY", config: "{", policyChange: true, exit: 1, want: []string{"required checks: 1 passed, 1 crossed, 1 unverified", "policy files changed: 1", "Repair unintended violations", "Resolve incomplete verification", "Review policy changes separately"}},
		{name: "passing proposal", mode: "preview", policyChange: true, exit: 0, want: []string{"PREVIEW / NOT APPROVED", "required checks: 3 passed, 0 crossed, 0 unverified", "policy files changed: 1", "preview does not approve adoption"}, absent: []string{"Repair unintended", "Resolve incomplete", "use preview"}},
		{name: "mixed proposal", mode: "preview", source: "DEBUG_ONLY", config: "{", policyChange: true, exit: 1, want: []string{"required checks: 1 passed, 1 crossed, 1 unverified", "Repair unintended violations", "Resolve incomplete verification", "preview does not approve adoption"}, absent: []string{"use preview"}},
		{name: "advisory issues", mode: "preview", source: "DEBUG_ONLY", config: "{", advisory: true, exit: 0, want: []string{"required checks: 1 passed, 0 crossed, 0 unverified", "advisory checks: 0 passed, 1 crossed, 1 unverified", "they do not block the exit code"}, absent: []string{"Repair unintended", "Resolve incomplete"}},
		{name: "fatal", mode: "check", exit: 2, want: []string{"no checks completed", "Resolve incomplete verification"}, absent: []string{"required checks:", "Re-run after relevant edits"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			mode := test.mode
			if mode == "" {
				mode = "check"
			}
			candidatePolicy := policy
			if test.policyChange {
				candidatePolicy += "\n# An intentional proposal\n"
			}
			if test.advisory {
				candidatePolicy = strings.ReplaceAll(candidatePolicy, "    scope:", "    required: false\n    scope:")
			}
			source, config := test.source, test.config
			if source == "" {
				source = "Allowed\n"
			}
			if config == "" {
				config = `{"enabled":true}`
			}
			write(t, filepath.Join(dir, ".linebind.yml"), candidatePolicy)
			write(t, filepath.Join(dir, "source.txt"), source)
			write(t, filepath.Join(dir, "config.json"), config)
			comparison := base
			if test.name == "fatal" {
				comparison = "missing-reference"
			}
			args := []string{mode, "--repo", dir, "--base", comparison}
			for _, agent := range []bool{false, true} {
				textArgs := append([]string{}, args...)
				if agent {
					textArgs = append(textArgs, "--agent")
				}
				output := cli(t, test.exit, textArgs...)
				for _, want := range append([]string{"Summary:", "Next action:"}, test.want...) {
					if !strings.Contains(output, want) {
						t.Errorf("missing %q in %s", want, output)
					}
				}
				for _, absent := range test.absent {
					if strings.Contains(output, absent) {
						t.Errorf("misleading guidance %q in %s", absent, output)
					}
				}
			}
			jsonOutput := cli(t, test.exit, append(args, "--format", "json")...)
			result, err := loadReport([]byte(jsonOutput))
			if err != nil || result.SchemaVersion != 2 || result.ExitCode != test.exit {
				t.Fatalf("JSON compatibility or verdict changed: %v\n%s", err, jsonOutput)
			}
			if strings.Contains(jsonOutput, "Summary:") || strings.Contains(jsonOutput, "Next action:") {
				t.Fatal("presentation-only guidance leaked into JSON")
			}
		})
	}
}
