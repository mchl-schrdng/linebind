package app

import "testing"

func TestMergeBaseRecordsSHAAndUsesAncestorPolicy(t *testing.T) {
	dir, opt := checkRepo(t, testContract, "allowed\n")
	ancestor := opt.Base
	testGit(t, dir, "branch", "approved")
	put(t, dir, ".linebind.yml", "invalid: [")
	put(t, dir, "src/code.txt", "banned\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-qm", "candidate")
	head := testGit(t, dir, "rev-parse", "HEAD")
	for _, candidate := range []string{"worktree", head} {
		opt.Base, opt.Head = "merge-base:approved", candidate
		got := Check(opt)
		if got.Base != ancestor || got.ExitCode != 1 || len(got.Changes) != 1 || len(got.Checks[0].Findings) != 1 {
			t.Fatalf("lost base policy or SHA: %+v", got)
		}
	}
	opt.Base = "merge-base:missing"
	if got := Check(opt); got.ExitCode != 2 {
		t.Fatalf("missing ancestor passed: %+v", got)
	}
}
