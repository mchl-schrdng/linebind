package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/model"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, b)
	}
}
func cli(t *testing.T, want int, args ...string) string {
	t.Helper()
	var out, err bytes.Buffer
	got := run(args, &out, &err)
	if got != want {
		t.Fatalf("%v: exit %d want %d\n%s\n%s", args, got, want, out.String(), err.String())
	}
	return out.String()
}
func write(t *testing.T, name, data string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCLIInitCheckAndOfflineReport(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	cli(t, 0, "init", "--repo", dir)
	cli(t, 2, "init", "--repo", dir)
	cli(t, 0, "validate", "--repo", dir)
	write(t, filepath.Join(dir, "README.md"), "# Project\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "approved")
	jsonPath := filepath.Join(dir, ".linebind", "report.json")
	htmlPath := filepath.Join(dir, ".linebind", "report.html")
	cli(t, 0, "check", "--repo", dir, "--base", "HEAD", "--format", "json", "--output", jsonPath, "--html", htmlPath)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var r model.Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != "passed" || r.InputDigest == "" || r.Base == "HEAD" {
		t.Fatalf("missing provenance: %+v", r)
	}
	if r.SchemaVersion != 2 || r.Checks[0].Specification == "" {
		t.Fatal("new reports must record evaluated rule definitions")
	}
	legacy := r
	legacy.SchemaVersion = 1
	legacy.Checks = append([]model.Check(nil), r.Checks...)
	for i := range legacy.Checks {
		legacy.Checks[i].Specification = ""
	}
	legacyData, _ := json.Marshal(legacy)
	if _, err := loadReport(legacyData); err != nil {
		t.Fatalf("legacy report rejected: %v", err)
	}
	legacy.SchemaVersion = 2
	missingSpec, _ := json.Marshal(legacy)
	if _, err := loadReport(missingSpec); err == nil {
		t.Fatal("schema 2 accepted missing definitions")
	}
	r.RuleChanges = []model.RuleChange{{ID: "x", Change: "modified", Before: "same", After: "same"}}
	badChange, _ := json.Marshal(r)
	if _, err := loadReport(badChange); err == nil {
		t.Fatal("accepted empty rule modification")
	}
	r.RuleChanges = nil
	// A second run replaces only recognized reports, and ignored outputs do not alter inputs.
	cli(t, 0, "check", "--repo", dir, "--base", "HEAD", "--format", "json", "--output", jsonPath, "--html", htmlPath)
	second, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var r2 model.Report
	json.Unmarshal(second, &r2)
	if r.InputDigest != r2.InputDigest {
		t.Fatal("report entered source snapshot")
	}
	cli(t, 0, "report", jsonPath, "--output", htmlPath)
	cli(t, 0, "report", "--output", htmlPath, jsonPath)
	html, err := os.ReadFile(htmlPath)
	if err != nil || !bytes.Contains(html, []byte("Required checks passed.")) {
		t.Fatalf("missing rendered report: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	cli(t, 1, "check", "--repo", dir, "--base", "HEAD", "--agent")
	cli(t, 2, "check", "--repo", dir)
	cli(t, 2, "check", "--repo", dir, "--base", "HEAD", "--format", "json", "--agent")
}

func TestInitRejectsContractCollidingWithGitignore(t *testing.T) {
	for _, name := range []string{".gitignore", ".GITIGNORE", ".gitignore/contract.yml"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			git(t, dir, "init", "-q")
			cli(t, 2, "init", "--repo", dir, "--contract", name)
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != ".git" {
				t.Fatalf("rejected init changed the repository: %v, %v", entries, err)
			}
		})
	}
}

func TestOutputsRefuseSourceAndValidateReportSchema(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	write(t, source, "valuable source")
	if err := writeArtifact(source, []byte("replacement"), "text"); err == nil {
		t.Fatal("overwrote source")
	}
	data, _ := os.ReadFile(source)
	if string(data) != "valuable source" {
		t.Fatal("source changed")
	}
	for _, data := range []string{`{}`, `{"schema_version":1,"schema_version":1}`, `{"schema_version":2,"version":"test"}`, `{"schema_version":1,"version":"test","generated_at":"now","status":"passed","exit_code":0,"checks":[]}`} {
		if _, err := loadReport([]byte(data)); err == nil {
			t.Fatalf("accepted invalid report %s", data)
		}
	}
	if !strings.Contains(cli(t, 0, "--help"), "--allow-exec") {
		t.Fatal("missing trust option")
	}
	cli(t, 2, "not-a-command")
}

func TestSavedReportRequiresCompleteEvidence(t *testing.T) {
	data := []byte(`{"schema_version":1,"version":"0.1.0","generated_at":"2026-10-14T19:45:36Z","status":"passed","exit_code":0,"checks":[{"id":"x","status":"passed","required":true}]}`)
	if _, err := loadReport(data); err == nil {
		t.Fatal("accepted success without provenance or evidence type")
	}
	// A captured operational error is still useful to render despite absent hashes.
	data = []byte(`{"schema_version":1,"version":"0.1.0","generated_at":"2026-10-14T19:45:36Z","status":"unverified","exit_code":2,"checks":[],"diagnostics":["base not available"]}`)
	if _, err := loadReport(data); err != nil {
		t.Fatal(err)
	}
}

func TestFIFOInputsAreRejectedBeforeOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix FIFO fixture")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	fifo := filepath.Join(dir, ".linebind.yml")
	if output, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, output)
	}
	cli(t, 2, "validate", "--repo", dir)
	if _, err := readLimited(fifo, 1024); err == nil {
		t.Fatal("accepted FIFO")
	}
}
