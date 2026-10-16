package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveMergeBase(t *testing.T) {
	dir := fixture(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	base := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "branch", "upstream")
	gitTest(t, dir, "commit", "--allow-empty", "-qm", "feature")
	feature := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "checkout", "-q", "upstream")
	gitTest(t, dir, "commit", "--allow-empty", "-qm", "upstream change")
	for _, head := range []string{feature, "feature-ref"} {
		if head == "feature-ref" {
			gitTest(t, dir, "branch", head, feature)
		}
		got, err := r.ResolveBase("merge-base:upstream", head)
		if err != nil || got != base {
			t.Fatalf("explicit candidate: %s %v", got, err)
		}
	}
	gitTest(t, dir, "checkout", "-q", "--detach", feature)
	for _, head := range []string{"", "worktree"} {
		got, err := r.ResolveBase("merge-base:upstream", head)
		if err != nil || got != base {
			t.Fatalf("worktree: %s %v", got, err)
		}
	}
	for _, ref := range []string{"merge-base:", "merge-base:missing", "merge-base:--help"} {
		if _, err := r.ResolveBase(ref, "HEAD"); err == nil {
			t.Fatalf("accepted %q", ref)
		}
	}
	if _, err := r.ResolveBase("merge-base:upstream", "missing"); err == nil {
		t.Fatal("accepted missing candidate")
	}
	// Build a criss-cross history without depending on merge conflict behavior.
	tree := gitTest(t, dir, "rev-parse", "HEAD^{tree}")
	left := gitTest(t, dir, "rev-parse", "upstream")
	a := gitTest(t, dir, "commit-tree", tree, "-p", feature, "-p", left, "-m", "merge A")
	b := gitTest(t, dir, "commit-tree", tree, "-p", left, "-p", feature, "-m", "merge B")
	if _, err := r.ResolveBase("merge-base:"+a, b); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous ancestor: %v", err)
	}
	unrelated := gitTest(t, dir, "commit-tree", tree, "-m", "other root")
	if _, err := r.ResolveBase("merge-base:"+unrelated, feature); err == nil {
		t.Fatal("accepted unrelated histories")
	}
	// A shallow repository may return a plausible but incomplete ancestor.
	if err := os.WriteFile(filepath.Join(dir, ".git", "shallow"), []byte(base+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ResolveBase("merge-base:upstream", feature); err == nil || !strings.Contains(err.Error(), "complete history") {
		t.Fatalf("shallow history: %v", err)
	}
	if got, err := r.ResolveBase(base, feature); err != nil || got != base {
		t.Fatalf("explicit SHA needs no history: %s %v", got, err)
	}
}
