package action

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestInstallerVerifiesBeforeExecuting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux action uses Unix tools")
	}
	for _, tool := range []string{"bash", "curl", "tar", "shasum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("missing " + tool)
		}
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "fixture.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\nprintf executed > \"$EXECUTED_MARKER\"\necho 'linebind fixture'\n")
	if err := tw.WriteHeader(&tar.Header{Name: "linebind", Mode: 0755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	mock := filepath.Join(dir, "curl")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$DOWNLOAD_ARGS"
while [ "$#" -gt 0 ]; do
  if [ "$1" = --output ]; then cp "$FIXTURE_ARCHIVE" "$2"; exit; fi
  shift
done
exit 1
`
	if err := os.WriteFile(mock, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, version, sha, os string
		pass, download         bool
	}{
		{"verified", "v0.3.0", checksum, "Linux", true, true},
		{"tampered", "v0.3.0", strings.Repeat("0", 64), "Linux", false, true},
		{"version injection", "v0.3.0/../../other", checksum, "Linux", false, false},
		{"checksum injection", "v0.3.0", checksum + "\n", "Linux", false, false},
		{"unsupported runner", "v0.3.0", checksum, "macOS", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "executed")
			args := filepath.Join(root, "args")
			output := filepath.Join(root, "output")
			path := filepath.Join(root, "path")
			cmd := exec.Command("bash", "install.sh")
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "RUNNER_OS="+tc.os, "RUNNER_ARCH=X64", "RUNNER_TEMP="+root, "GITHUB_PATH="+path, "GITHUB_OUTPUT="+output, "LINEBIND_VERSION="+tc.version, "LINEBIND_SHA256="+tc.sha, "FIXTURE_ARCHIVE="+archive, "EXECUTED_MARKER="+marker, "DOWNLOAD_ARGS="+args)
			log, err := cmd.CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("unexpected exit: %v %s", err, log)
			}
			_, executed := os.Stat(marker)
			if (executed == nil) != tc.pass {
				t.Fatalf("execution before verification: %s", log)
			}
			got, err := os.ReadFile(args)
			if (err == nil) != tc.download {
				t.Fatal("unexpected download")
			}
			if tc.download && !strings.Contains(string(got), "https://github.com/mchl-schrdng/linebind/releases/download/v0.3.0/linebind_0.3.0_linux_amd64.tar.gz") {
				t.Fatal("unexpected download URL")
			}
			if tc.pass {
				got, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				binary := strings.TrimSpace(strings.TrimPrefix(string(got), "binary="))
				if _, err := os.Stat(binary); err != nil {
					t.Fatal(err)
				}
			} else if _, err := os.Stat(output); err == nil {
				t.Fatal("failed installation exposed an executable")
			}
		})
	}
}

func TestActionPinsPublishedDefaults(t *testing.T) {
	data, err := os.ReadFile("../../action.yml")
	if err != nil {
		t.Fatal(err)
	}
	var action struct {
		Inputs map[string]struct {
			Default string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile("../../.github/workflows/contract.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "sha256"} {
		value := action.Inputs[key].Default
		if value == "" || !strings.Contains(string(workflow), value) {
			t.Fatalf("%s default must match the trusted CI verifier", key)
		}
	}
}
