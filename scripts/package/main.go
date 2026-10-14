// Command package builds the six standalone release archives without external tools.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type archiveFile struct {
	name string
	data []byte
	mode int64
}

func main() {
	version := flag.String("version", "", "release tag, for example v0.1.0")
	commit := flag.String("commit", "", "full commit SHA (defaults to HEAD)")
	out := flag.String("out", "dist", "new output directory; must not already exist")
	flag.Parse()
	if *commit == "" {
		data, err := exec.Command("git", "rev-parse", "--verify", "HEAD").Output()
		if err != nil {
			fail(fmt.Errorf("resolve commit: %w", err))
		}
		*commit = strings.TrimSpace(string(data))
	}
	if err := buildRelease(*version, *commit, *out); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func buildRelease(version, commit, out string) error {
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("version must be an exact stable tag vMAJOR.MINOR.PATCH")
	}
	if !commitPattern.MatchString(commit) {
		return fmt.Errorf("commit must be a full lowercase 40-character Git SHA")
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		return err
	}
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	notices, err := os.ReadFile("THIRD_PARTY_NOTICES")
	if err != nil {
		return err
	}
	if err := os.Mkdir(out, 0o755); err != nil {
		return fmt.Errorf("create release directory: %w", err)
	}
	tmp, err := os.MkdirTemp("", "linebind-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var checksums strings.Builder
	for _, platform := range []string{"darwin", "linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			binary, extension := "linebind", ".tar.gz"
			if platform == "windows" {
				binary, extension = "linebind.exe", ".zip"
			}
			binaryPath := filepath.Join(tmp, binary)
			flags := "-s -w -X main.version=" + version + " -X main.commit=" + commit
			cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", flags, "-o", binaryPath, "./cmd/linebind")
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+platform, "GOARCH="+arch)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("build %s/%s: %w", platform, arch, err)
			}
			data, err := os.ReadFile(binaryPath)
			if err != nil {
				return err
			}
			files := []archiveFile{{binary, data, 0o755}, {"README.md", readme, 0o644}, {"LICENSE", license, 0o644}, {"THIRD_PARTY_NOTICES", notices, 0o644}}
			packed, err := pack(files, platform == "windows")
			if err != nil {
				return err
			}
			name := "linebind_" + strings.TrimPrefix(version, "v") + "_" + platform + "_" + arch + extension
			if err := os.WriteFile(filepath.Join(out, name), packed, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(packed), name)
			fmt.Println(name)
		}
	}
	return os.WriteFile(filepath.Join(out, "SHA256SUMS"), []byte(checksums.String()), 0o644)
}

func pack(files []archiveFile, windows bool) ([]byte, error) {
	var buffer bytes.Buffer
	// Fixed metadata keeps archive hashes independent of checkout times and ownership.
	stamp := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	if windows {
		writer := zip.NewWriter(&buffer)
		for _, file := range files {
			header := &zip.FileHeader{Name: file.name, Method: zip.Deflate, Modified: stamp}
			header.SetMode(os.FileMode(file.mode))
			entry, err := writer.CreateHeader(header)
			if err != nil {
				return nil, err
			}
			if _, err := entry.Write(file.data); err != nil {
				return nil, err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
	} else {
		compressed := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(compressed)
		for _, file := range files {
			header := &tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.data)), ModTime: stamp, Typeflag: tar.TypeReg}
			if err := writer.WriteHeader(header); err != nil {
				return nil, err
			}
			if _, err := writer.Write(file.data); err != nil {
				return nil, err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		if err := compressed.Close(); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}
