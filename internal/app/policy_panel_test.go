package app

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mchl-schrdng/linebind/internal/contract"
	"github.com/mchl-schrdng/linebind/internal/repository"
)

func loadExampleSnapshot(t *testing.T, name, variant string) *repository.Snapshot {
	t.Helper()
	root := filepath.Join("..", "..", "examples", name, variant)
	snapshot := &repository.Snapshot{Files: map[string][]byte{}}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			snapshot.Files[filepath.ToSlash(relative)] = data
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestRealPodPolicyPanel(t *testing.T) {
	bin := os.Getenv("LINEBIND_CONFTEST")
	if bin == "" {
		t.Skip("set LINEBIND_CONFTEST to pinned Conftest 0.62.0 executable")
	}
	base := loadExampleSnapshot(t, "conftest", "base")
	config, err := contract.Load(base.Files[".linebind.yml"])
	if err != nil {
		t.Fatal(err)
	}
	rule := config.Rules[0]
	rule.Command.Argv[0] = bin
	pod := func(containers, extra string) string {
		return fmt.Sprintf(`{"apiVersion":"v1","kind":"Pod","spec":{"containers":%s%s}}`, containers, extra)
	}
	valid := pod(`[{"name":"app","securityContext":{"privileged":false}}]`, "")
	privileged := pod(`[{"name":"app","securityContext":{"privileged":true}}]`, "")
	for _, tc := range []struct{ name, input, status string }{
		{"base", string(base.Files["infra/pod.yaml"]), "passed"},
		{"image-change", strings.ReplaceAll(string(base.Files["infra/pod.yaml"]), "app:1.0", "app:2.0"), "passed"},
		{"omitted-privileged", pod(`[{"name":"app"}]`, ""), "passed"},
		{"regular-privileged", privileged, "crossed"},
		{"init-privileged", pod(`[{"name":"app"}]`, `,"initContainers":[{"name":"init","securityContext":{"privileged":true}}]`), "crossed"},
		{"ephemeral-privileged", pod(`[{"name":"app"}]`, `,"ephemeralContainers":[{"name":"debug","securityContext":{"privileged":true}}]`), "crossed"},
		{"missing-name-cannot-hide-violation", pod(`[{"securityContext":{"privileged":true}}]`, ""), "crossed"},
		{"string-privileged", pod(`[{"securityContext":{"privileged":"true"}}]`, ""), "crossed"},
		{"null-privileged", pod(`[{"securityContext":{"privileged":null}}]`, ""), "crossed"},
		{"invalid-security-context", pod(`[{"securityContext":false}]`, ""), "crossed"},
		{"missing-spec", `{"apiVersion":"v1","kind":"Pod"}`, "crossed"},
		{"empty-containers", pod(`[]`, ""), "crossed"},
		{"invalid-container-group", pod(`[{"name":"app"}]`, `,"initContainers":{}`), "crossed"},
		{"unsupported-kind", strings.ReplaceAll(privileged, `"Pod"`, `"Deployment"`), "crossed"},
		{"multiple-documents", valid + "\n---\n" + privileged, "crossed"},
		{"empty-object", `{}`, "crossed"},
		{"malformed-yaml", "spec: [\n", "unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := &repository.Snapshot{Files: map[string][]byte{}}
			for path, data := range base.Files {
				candidate.Files[path] = data
			}
			candidate.Files["infra/pod.yaml"] = []byte(tc.input)
			if got := runCommand(rule, candidate, base, true); got.Status != tc.status {
				t.Fatalf("Pod policy %s: %+v", tc.name, got)
			}
		})
	}
	t.Run("additional-yml-file", func(t *testing.T) {
		candidate := loadExampleSnapshot(t, "conftest", "base")
		candidate.Files["infra/nested/new.yml"] = []byte(privileged)
		if got := runCommand(rule, candidate, base, true); got.Status != "crossed" {
			t.Fatalf("new .yml Pod was not covered: %+v", got)
		}
	})
}

func TestRealDataPolicyPanel(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for the SQLite example")
	}
	base := loadExampleSnapshot(t, "data-command", "base")
	config, err := contract.Load(base.Files[".linebind.yml"])
	if err != nil {
		t.Fatal(err)
	}
	rule := config.Rules[0]
	for _, tc := range []struct{ name, sql, status string }{
		{"base", string(base.Files["transform.sql"]), "passed"},
		{"reordered", "SELECT order_id, customer_id FROM input_orders ORDER BY order_id DESC", "passed"},
		{"duplicate-keys", "SELECT 101 AS order_id, customer_id FROM input_orders", "crossed"},
		{"null-keys", "SELECT NULL AS order_id, customer_id FROM input_orders", "crossed"},
		{"empty-output", "SELECT order_id, customer_id FROM input_orders WHERE 0", "crossed"},
		{"dropped-order", "SELECT order_id, customer_id FROM input_orders WHERE order_id != 103", "crossed"},
		{"invented-keys", "SELECT order_id + 1000 AS order_id, customer_id FROM input_orders", "crossed"},
		{"wrong-customer", "SELECT order_id, 'wrong' AS customer_id FROM input_orders", "crossed"},
		{"null-customer", "SELECT order_id, NULL AS customer_id FROM input_orders", "crossed"},
		{"malformed-sql", "SELEC order_id FROM input_orders", "unverified"},
		{"write-attempt", "DELETE FROM input_orders", "unverified"},
		{"wrong-columns", "SELECT order_id FROM input_orders", "unverified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := &repository.Snapshot{Files: map[string][]byte{}}
			for path, data := range base.Files {
				candidate.Files[path] = data
			}
			candidate.Files["transform.sql"] = []byte(tc.sql)
			if got := runCommand(rule, candidate, base, true); got.Status != tc.status {
				t.Fatalf("data policy %s: %+v", tc.name, got)
			}
		})
	}
}
