package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
)

func TestSetupCLIAndDryRun(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	cli(t, 2, "setup", "--repo", dir)
	cli(t, 2, "setup", "--repo", dir, "--agent", "unknown")
	cli(t, 2, "setup", "--repo", dir, "--agent", "all", "extra")
	cli(t, 0, "setup", "--repo", dir, "--agent", "all", "--dry-run")
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote instructions")
	}
	cli(t, 0, "setup", "--repo", dir, "--agent", "all")
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", ".agents/skills/linebind/SKILL.md", ".claude/skills/linebind/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".linebind.yml")); !os.IsNotExist(err) {
		t.Fatal("setup invented a contract")
	}
	cli(t, 0, "setup", "--repo", dir, "--agent", "all")
}

func TestPreviewCLIProducesDistinctRoundTrippableReport(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "README.md"), "# Frontend\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "project before its first contract")
	cli(t, 0, "init", "--repo", dir)
	cli(t, 2, "preview", "--repo", dir)
	cli(t, 2, "preview", "--repo", dir, "--base", "HEAD", "--format", "json", "--agent")
	jsonPath := filepath.Join(dir, ".linebind", "preview.json")
	htmlPath := filepath.Join(dir, ".linebind", "preview.html")
	cli(t, 0, "preview", "--repo", dir, "--base", "HEAD", "--format", "json", "--output", jsonPath, "--html", htmlPath)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := loadReport(data)
	if err != nil || r.Purpose != "preview" || r.Status != "passed" || len(r.Changes) != 1 || r.Changes[0].Change != "added" {
		t.Fatalf("invalid bootstrap preview: %s (%v)", data, err)
	}
	// Saving twice must recognize the preview as an artifact, not reject or
	// silently reclassify it as an approved-policy check.
	cli(t, 0, "preview", "--repo", dir, "--base", "HEAD", "--format", "json", "--output", jsonPath)
	cli(t, 0, "report", jsonPath, "--output", htmlPath)
	html, err := os.ReadFile(htmlPath)
	if err != nil || !strings.Contains(strings.ToLower(string(html)), "preview") || !strings.Contains(strings.ToLower(string(html)), "not approved") {
		t.Fatalf("preview distinction missing from rendered report: %v", err)
	}
	cli(t, 2, "check", "--repo", dir, "--base", "HEAD")
	r.Purpose = "approved"
	invalid, _ := json.Marshal(r)
	if _, err := loadReport(invalid); err == nil {
		t.Fatal("accepted unknown purpose")
	}
	r.Purpose = ""
	invalid, _ = json.Marshal(r)
	if _, err := loadReport(invalid); err == nil {
		t.Fatal("accepted passing check with unapproved policy changes")
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "reviewed contract")
	checked := cli(t, 0, "check", "--repo", dir, "--base", "HEAD", "--format", "json")
	var report model.Report
	if err := json.Unmarshal([]byte(checked), &report); err != nil || report.Purpose != "" || strings.Contains(checked, `"purpose"`) {
		t.Fatalf("legacy check format changed: %s", checked)
	}
}
