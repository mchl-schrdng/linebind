package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestStableReleaseTag(t *testing.T) {
	for tag, valid := range map[string]bool{
		"v0.1.0": true, "v1.20.3": true, "v0.0.0": true,
		"v01.2.3": false, "v1.2": false, "1.2.3": false,
		"v1.2.3-rc.1": false, "v1.2.3+build": false, "v1.2.3/../x": false,
	} {
		if got := versionPattern.MatchString(tag); got != valid {
			t.Errorf("tag %q: accepted=%v, want %v", tag, got, valid)
		}
	}
}

func TestArchivesPreserveContentsAndPermissions(t *testing.T) {
	files := []archiveFile{{"linebind", []byte("binary"), 0o755}, {"README.md", []byte("readme"), 0o644}, {"LICENSE", []byte("license"), 0o644}}
	for _, windows := range []bool{false, true} {
		data, err := pack(files, windows)
		if err != nil {
			t.Fatal(err)
		}
		again, err := pack(files, windows)
		if err != nil || !bytes.Equal(data, again) {
			t.Fatal("archive bytes must be deterministic for identical inputs")
		}
		if windows {
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.File) != len(files) {
				t.Fatalf("zip has %d entries, want %d", len(reader.File), len(files))
			}
			for i, entry := range reader.File {
				r, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(r)
				r.Close()
				if err != nil || entry.Name != files[i].name || int64(entry.Mode().Perm()) != files[i].mode || !bytes.Equal(content, files[i].data) {
					t.Fatalf("zip entry %q did not preserve its content, name or permissions", entry.Name)
				}
			}
		} else {
			compressed, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			reader := tar.NewReader(compressed)
			for _, file := range files {
				header, err := reader.Next()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(reader)
				if err != nil || header.Name != file.name || header.Mode != file.mode || !bytes.Equal(content, file.data) {
					t.Fatalf("tar entry %q did not preserve its content, name or permissions", header.Name)
				}
			}
			if _, err := reader.Next(); err != io.EOF {
				t.Fatalf("unexpected trailing tar entry: %v", err)
			}
			compressed.Close()
		}
	}
}
