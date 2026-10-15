package agentsetup

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repository with spaces")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	return dir
}

func put(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func absent(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("%s should be absent; got %v", name, err)
		}
	}
}

func TestAllDryRunInstallAndIdempotenceWithoutInitialCommit(t *testing.T) {
	dir := newRepo(t)
	plan, err := Setup(dir, "all", true)
	if err != nil {
		t.Fatal(err)
	}
	absent(t, dir, ".agents", ".claude", "AGENTS.md", "CLAUDE.md", ".linebind.yml", ".gitignore")
	installed, err := Setup(dir, "all", false)
	if err != nil || !reflect.DeepEqual(plan, installed) {
		t.Fatalf("install differs from plan: %v\n%+v\n%+v", err, plan, installed)
	}
	seen := map[string]bool{}
	before := map[string][]byte{}
	for _, change := range installed {
		if seen[change.Path] || change.Action != "create" {
			t.Fatalf("duplicate or unexpected action: %+v", change)
		}
		seen[change.Path] = true
		before[change.Path] = get(t, dir, change.Path)
	}
	err = fs.WalkDir(skill, "skill", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		want, err := skill.ReadFile(name)
		if err != nil {
			return err
		}
		for _, target := range []string{".agents", ".claude"} {
			name := target + "/skills/linebind" + strings.TrimPrefix(name, "skill")
			if !bytes.Equal(get(t, dir, name), want) {
				t.Fatalf("%s does not match the bundled skill", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"all", "codex", "antigravity", "claude"} {
		changes, err := Setup(dir, agent, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range changes {
			if change.Action != "unchanged" || !bytes.Equal(get(t, dir, change.Path), before[change.Path]) {
				t.Fatalf("%s changed an installed file: %+v", agent, change)
			}
		}
	}
	absent(t, dir, ".linebind.yml", ".gitignore")
}

func TestPreservesGuidanceAndContractFromSubdirectory(t *testing.T) {
	dir := newRepo(t)
	texts := map[string]string{
		"AGENTS.md":     "# Existing project rules\nKeep this exact content without a final newline",
		"CLAUDE.md":     "# Claude conventions\r\n\r\nPreserve CRLF.\r\n",
		".linebind.yml": "custom contract preserved verbatim\n",
		".gitignore":    "private-data/\n",
	}
	for name, content := range texts {
		put(t, dir, name, content)
	}
	subdir := filepath.Join(dir, "nested")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	changes, err := Setup(subdir, "all", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		data := get(t, dir, name)
		if !bytes.HasPrefix(data, []byte(texts[name])) || bytes.Count(data, []byte(blockStart)) != 1 {
			t.Fatalf("%s was not preserved: %s", name, data)
		}
		if !bytes.Contains(data, []byte(".linebind.yml")) || !bytes.Contains(data, []byte("--base <base-sha>")) {
			t.Fatalf("missing contract guidance in %s", name)
		}
		if name == "CLAUDE.md" && bytes.Contains(data, []byte("@AGENTS.md")) {
			t.Fatal("Claude setup unexpectedly imports all AGENTS.md instructions")
		}
	}
	for _, change := range changes {
		if (change.Path == "AGENTS.md" || change.Path == "CLAUDE.md") && change.Action != "append" {
			t.Fatalf("expected append: %+v", change)
		}
	}
	for _, name := range []string{".linebind.yml", ".gitignore"} {
		if string(get(t, dir, name)) != texts[name] {
			t.Fatalf("%s changed", name)
		}
	}
	if _, err := Setup(dir, "all", false); err != nil {
		t.Fatalf("CRLF installation not idempotent: %v", err)
	}
	absent(t, subdir, ".agents", ".claude", "AGENTS.md", "CLAUDE.md")
}

func TestInstallAfterCRLFCheckout(t *testing.T) {
	dir := newRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		if output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	runGit("config", "core.autocrlf", "true")
	runGit("config", "core.safecrlf", "false")
	runGit("config", "core.hooksPath", t.TempDir())
	installed, err := Setup(dir, "codex", false)
	if err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "Installed skill")
	for _, file := range installed {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(file.Path))); err != nil {
			t.Fatal(err)
		}
	}
	runGit("checkout", "--", ".")
	before := map[string][]byte{}
	for _, file := range installed {
		before[file.Path] = get(t, dir, file.Path)
		if !bytes.Contains(before[file.Path], []byte("\r\n")) {
			t.Fatalf("%s was not checked out with CRLF", file.Path)
		}
	}
	changes, err := Setup(dir, "all", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if previous, exists := before[change.Path]; exists {
			if change.Action != "unchanged" || !bytes.Equal(get(t, dir, change.Path), previous) {
				t.Fatalf("CRLF file was rewritten: %+v", change)
			}
		} else if change.Action != "create" {
			t.Fatalf("additional agent was not installed: %+v", change)
		}
	}
}

func TestTargetSelection(t *testing.T) {
	for _, agent := range []string{"codex", "antigravity", "claude"} {
		t.Run(agent, func(t *testing.T) {
			dir := newRepo(t)
			if _, err := Setup(dir, agent, false); err != nil {
				t.Fatal(err)
			}
			if agent == "claude" {
				get(t, dir, ".claude/skills/linebind/SKILL.md")
				absent(t, dir, ".agents", "AGENTS.md")
			} else {
				get(t, dir, ".agents/skills/linebind/SKILL.md")
				absent(t, dir, ".claude", "CLAUDE.md")
			}
		})
	}
}

func TestConflictsRejectWholePlan(t *testing.T) {
	for _, fixture := range []struct{ path, content string }{
		{".claude/skills/linebind/SKILL.md", "my customized skill"},
		{"CLAUDE.md", blockStart + "\ncustom instructions\n" + blockEnd},
		{"CLAUDE.md", blockStart + "\nunfinished block"},
		{"CLAUDE.md", blockEnd + "\n" + blockStart},
		{"CLAUDE.md", instructionBlock(".claude/skills/linebind") + instructionBlock(".claude/skills/linebind")},
	} {
		t.Run(fixture.path+fixture.content[:3], func(t *testing.T) {
			dir := newRepo(t)
			put(t, dir, fixture.path, fixture.content)
			for _, dryRun := range []bool{true, false} {
				if _, err := Setup(dir, "all", dryRun); err == nil {
					t.Fatal("accepted conflict")
				}
				absent(t, dir, ".agents", "AGENTS.md")
				if string(get(t, dir, fixture.path)) != fixture.content {
					t.Fatal("conflicting content changed")
				}
			}
		})
	}
}

func TestSymlinkTargetsAndParentsAreRejected(t *testing.T) {
	for _, name := range []string{".agents", ".agents/skills", ".agents/skills/linebind", ".agents/skills/linebind/references", ".agents/skills/linebind/SKILL.md", "AGENTS.md", "CLAUDE.md"} {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			target := filepath.Join(dir, "untouched")
			if strings.HasSuffix(name, ".md") {
				put(t, dir, "untouched", "existing content")
			} else if err := os.Mkdir(target, 0755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink creation unavailable: %v", err)
				}
				t.Fatal(err)
			}
			if _, err := Setup(dir, "all", false); err == nil {
				t.Fatal("accepted symlink")
			}
			absent(t, dir, ".claude")
			if strings.HasSuffix(name, ".md") {
				if string(get(t, dir, "untouched")) != "existing content" {
					t.Fatal("symlink target changed")
				}
			} else if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
				t.Fatalf("symlink directory changed: %v", err)
			}
		})
	}
}

func TestNonRegularDestinationsAndInvalidRequests(t *testing.T) {
	for _, name := range []string{"AGENTS.md", ".agents/skills/linebind/SKILL.md"} {
		dir := newRepo(t)
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(name)), 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Setup(dir, "all", false); err == nil {
			t.Fatalf("accepted directory at file destination %s", name)
		}
		absent(t, dir, ".claude", "CLAUDE.md")
	}
	dir := newRepo(t)
	for _, agent := range []string{"", "unknown", "../claude"} {
		if _, err := Setup(dir, agent, false); err == nil {
			t.Fatalf("accepted agent %q", agent)
		}
	}
	if _, err := Setup(t.TempDir(), "all", false); err == nil {
		t.Fatal("accepted non-Git directory")
	}
	absent(t, dir, ".agents", ".claude", "AGENTS.md", "CLAUDE.md")
}

func TestFIFORejectedBeforeOpening(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix FIFO fixture")
	}
	dir := newRepo(t)
	if output, err := exec.Command("mkfifo", filepath.Join(dir, "CLAUDE.md")).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, output)
	}
	if _, err := Setup(dir, "all", false); err == nil {
		t.Fatal("accepted FIFO")
	}
	absent(t, dir, ".agents", ".claude", "AGENTS.md")
}

func TestPlannedWriteRefusesChangedDestinations(t *testing.T) {
	dir := newRepo(t)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	put(t, dir, "AGENTS.md", "a concurrent edit")
	for _, file := range []plannedFile{
		{Change{"AGENTS.md", "create"}, []byte("new instructions"), nil},
		{Change{"AGENTS.md", "append"}, []byte("new instructions"), []byte("previous instructions")},
	} {
		if err := writePlanned(root, file); err == nil {
			t.Fatal("changed destination accepted")
		}
		if string(get(t, dir, "AGENTS.md")) != "a concurrent edit" {
			t.Fatal("concurrent edit changed")
		}
	}
}
