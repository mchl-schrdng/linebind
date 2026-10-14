package repository

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGitOutputBoundCannotBeBypassedByCopy(t *testing.T) {
	b := &boundedBuffer{limit: 8}
	reader := struct{ io.Reader }{strings.NewReader("more than eight bytes")}
	if _, err := io.Copy(b, reader); err == nil || !b.overflow || len(b.Bytes()) > 8 {
		t.Fatal("io.Copy bypassed the output limit")
	}
}

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo with spaces")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "code.txt"), []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "base")
	return dir
}

func TestSnapshotsAndWorkingTreeDoNotChangeGitState(t *testing.T) {
	dir := fixture(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, err := r.Commit("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "code.txt"), []byte("changed\n"), 0600)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("untracked\n"), 0600)
	before := gitTest(t, dir, "status", "--porcelain=v1")
	s, err := r.WorkingTree()
	if err != nil {
		t.Fatal(err)
	}
	if string(s.Files["code.txt"]) != "changed\n" || string(s.Files["new.txt"]) != "untracked\n" {
		t.Fatalf("wrong working tree: %#v", s.Files)
	}
	if string(base.Files["code.txt"]) != "original\n" || base.Digest == s.Digest {
		t.Fatal("snapshot not immutable")
	}
	if !strings.HasPrefix(s.Identity, "worktree:") {
		t.Fatal("worktree pretends to be a commit")
	}
	if after := gitTest(t, dir, "status", "--porcelain=v1"); after != before {
		t.Fatalf("repository modified: %q != %q", before, after)
	}
	os.Remove(filepath.Join(dir, "code.txt"))
	deleted, err := r.WorkingTree()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := deleted.Files["code.txt"]; ok {
		t.Fatal("tracked deletion not captured")
	}
}

func TestCommitIgnoresExportIgnoreAndSupportsWorktrees(t *testing.T) {
	dir := fixture(t)
	os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("code.txt export-ignore\n"), 0600)
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "attributes")
	linked := filepath.Join(t.TempDir(), "linked checkout")
	gitTest(t, dir, "worktree", "add", "--detach", linked, "HEAD")
	r, err := Open(linked)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Commit("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Files["code.txt"]; !ok {
		t.Fatal("export-ignore hid source")
	}
	if _, err := r.WorkingTree(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("--help"); err == nil {
		t.Fatal("option accepted as revision")
	}
	if _, err := r.Resolve("missing-object"); err == nil {
		t.Fatal("missing base accepted")
	}
}

func TestUnsafeAndUnavailableInputs(t *testing.T) {
	for _, p := range []string{"../escape", "/absolute", "a/../b", ".git/config", "x/.GIT/config", "a\\b", "C:/path", "a\x00b"} {
		if SafePath(p) {
			t.Fatalf("accepted unsafe path %q", p)
		}
	}
	dir := fixture(t)
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		gitTest(t, dir, "add", "link")
		gitTest(t, dir, "commit", "-qm", "symlink")
		r, _ := Open(dir)
		s, err := r.Commit("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if s.Omitted["link"] == "" {
			t.Fatal("symlink silently accepted")
		}
		s, err = r.WorkingTree()
		if err != nil {
			t.Fatal(err)
		}
		if s.Omitted["link"] == "" {
			t.Fatal("working symlink silently accepted")
		}
	}
	os.WriteFile(filepath.Join(dir, "large.dat"), make([]byte, MaxFileBytes+1), 0600)
	r, _ := Open(dir)
	s, err := r.WorkingTree()
	if err != nil {
		t.Fatal(err)
	}
	if s.Omitted["large.dat"] == "" {
		t.Fatal("large file not recorded")
	}
	s2 := emptySnapshot("test")
	s2.add("lfs", []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\n"))
	if s2.Omitted["lfs"] == "" {
		t.Fatal("LFS pointer treated as source")
	}
}

func TestMaterializePreservesContentAndCannotEscape(t *testing.T) {
	s := emptySnapshot("test")
	s.Files["sub/file"] = []byte("evidence")
	dir := t.TempDir()
	if err := s.Materialize(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "sub/file"))
	if err != nil || string(b) != "evidence" {
		t.Fatal("materialize lost input")
	}
	if err := s.Materialize(dir); err == nil {
		t.Fatal("materialize may overwrite an existing file or case-colliding path")
	}
	s.Files["../escape"] = []byte("no")
	if err := s.Materialize(t.TempDir()); err == nil {
		t.Fatal("unsafe output allowed")
	}
}

func TestIgnoredSymlinkParentDoesNotImportTrackedChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires separate Windows privileges")
	}
	dir := fixture(t)
	if err := os.Mkdir(filepath.Join(dir, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src/a.txt"), []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "source")
	if err := os.Rename(filepath.Join(dir, "src"), filepath.Join(dir, "real")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real/b.txt"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/src\n/real\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "src")); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.WorkingTree()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Files["src/a.txt"]; ok {
		t.Fatal("followed ignored symlink parent")
	}
	if s.Omitted["src"] == "" {
		t.Fatal("parent omission invisible")
	}
}
