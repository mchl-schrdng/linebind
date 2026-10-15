// Package repository captures Git inputs without changing the source checkout.
package repository

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxFileBytes  = 4 << 20
	MaxTotalBytes = 128 << 20
	MaxFiles      = 25000
)

type Snapshot struct {
	Identity   string
	Digest     string
	Files      map[string][]byte
	Executable map[string]bool
	Omitted    map[string]string
}

type Repository struct{ Root string }

func Open(dir string) (*Repository, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	r := &Repository{Root: abs}
	out, err := r.git(nil, 1<<20, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("open Git repository: %w", err)
	}
	r.Root = strings.TrimSuffix(strings.TrimSuffix(string(out), "\n"), "\r")
	return r, nil
}

func (r *Repository) Resolve(ref string) (string, error) {
	if ref == "" || strings.ContainsAny(ref, "\x00\r\n") {
		return "", fmt.Errorf("a nonempty Git revision is required")
	}
	b, err := r.git(nil, 4096, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve %q (no automatic fetch): %w", ref, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func SafePath(p string) bool {
	if p == "" || !utf8.ValidString(p) || strings.ContainsAny(p, "\\\x00\r\n") || path.IsAbs(p) || path.Clean(p) != p {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.Contains(part, ":") {
			return false
		}
	}
	return true
}

func emptySnapshot(identity string) *Snapshot {
	return &Snapshot{Identity: identity, Files: map[string][]byte{}, Executable: map[string]bool{}, Omitted: map[string]string{}}
}

func (r *Repository) Commit(ref string) (*Snapshot, error) {
	sha, err := r.Resolve(ref)
	if err != nil {
		return nil, err
	}
	tree, err := r.git(nil, 16<<20, "ls-tree", "-r", "-l", "-z", "--full-tree", sha)
	if err != nil {
		return nil, err
	}
	s := emptySnapshot(sha)
	type blob struct {
		name, hash string
		size       int
	}
	var blobs []blob
	var input bytes.Buffer
	entries, total := 0, 0
	for _, entry := range bytes.Split(tree, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		entries++
		if entries > MaxFiles {
			return nil, fmt.Errorf("snapshot exceeds %d files", MaxFiles)
		}
		parts := bytes.SplitN(entry, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Git tree entry")
		}
		name := string(parts[1])
		if !SafePath(name) {
			return nil, fmt.Errorf("unsupported repository path %q", name)
		}
		meta := strings.Fields(string(parts[0]))
		if len(meta) != 4 {
			return nil, fmt.Errorf("invalid Git tree metadata")
		}
		if meta[0] != "100644" && meta[0] != "100755" {
			s.Omitted[name] = "symlinks and submodules are not inspected"
			continue
		}
		size, err := strconv.Atoi(meta[3])
		if err != nil || size < 0 || meta[1] != "blob" {
			return nil, fmt.Errorf("invalid Git blob metadata")
		}
		if size > MaxFileBytes {
			s.Omitted[name] = fmt.Sprintf("file exceeds %d bytes", MaxFileBytes)
			continue
		}
		total += size
		if total > MaxTotalBytes {
			return nil, fmt.Errorf("snapshot exceeds %d bytes", MaxTotalBytes)
		}
		blobs = append(blobs, blob{name, meta[2], size})
		s.Executable[name] = meta[0] == "100755"
		fmt.Fprintln(&input, meta[2])
	}
	if len(blobs) > 0 {
		data, err := r.git(input.Bytes(), MaxTotalBytes+(8<<20), "cat-file", "--batch")
		if err != nil {
			return nil, fmt.Errorf("read Git blobs: %w", err)
		}
		reader := bufio.NewReader(bytes.NewReader(data))
		for _, blob := range blobs {
			header, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			parts := strings.Fields(header)
			if len(parts) != 3 || parts[0] != blob.hash || parts[1] != "blob" {
				return nil, fmt.Errorf("invalid blob response for %s", blob.name)
			}
			size, err := strconv.Atoi(parts[2])
			if err != nil || size != blob.size {
				return nil, fmt.Errorf("invalid blob size")
			}
			content := make([]byte, size)
			if _, err := io.ReadFull(reader, content); err != nil {
				return nil, err
			}
			if sep, err := reader.ReadByte(); err != nil || sep != '\n' {
				return nil, fmt.Errorf("invalid blob delimiter")
			}
			s.add(blob.name, content)
		}
	}
	s.Digest = s.digest()
	return s, nil
}

func (r *Repository) WorkingTree() (*Snapshot, error) {
	head, err := r.Resolve("HEAD")
	if err != nil {
		return nil, fmt.Errorf("working tree requires an initial commit: %w", err)
	}
	names, err := r.git(nil, 16<<20, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(r.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	s := emptySnapshot("worktree:" + head)
	seen := map[string]bool{}
	parents := map[string]bool{}
	indexExecutable := map[string]bool{}
	if runtime.GOOS == "windows" {
		index, err := r.git(nil, 16<<20, "ls-files", "--stage", "-z")
		if err != nil {
			return nil, err
		}
		for _, entry := range bytes.Split(index, []byte{0}) {
			if len(entry) == 0 {
				continue
			}
			parts := bytes.SplitN(entry, []byte{'\t'}, 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid Git index entry")
			}
			meta := strings.Fields(string(parts[0]))
			if len(meta) != 3 || meta[2] != "0" {
				return nil, fmt.Errorf("resolve index conflicts before capturing Windows executable flags")
			}
			indexExecutable[string(parts[1])] = meta[0] == "100755"
		}
	}
	total := 0
	for _, raw := range bytes.Split(names, []byte{0}) {
		name := string(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if len(seen) > MaxFiles {
			return nil, fmt.Errorf("snapshot exceeds %d files", MaxFiles)
		}
		if !SafePath(name) {
			return nil, fmt.Errorf("unsupported repository path %q", name)
		}
		readable, err := inspectParents(root, name, parents, s.Omitted)
		if err != nil {
			return nil, err
		}
		if !readable {
			continue
		}
		info, err := root.Lstat(name)
		if os.IsNotExist(err) {
			continue
		} // tracked deletion is part of the candidate
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			s.Omitted[name] = "symlinks, submodules and special files are not inspected"
			continue
		}
		if info.Size() > MaxFileBytes {
			s.Omitted[name] = fmt.Sprintf("file exceeds %d bytes", MaxFileBytes)
			continue
		}
		f, err := root.Open(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		content, readErr := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
		after, statErr := f.Stat()
		f.Close()
		if readErr != nil {
			return nil, readErr
		}
		if statErr != nil || !os.SameFile(info, after) || info.Size() != int64(len(content)) || !info.ModTime().Equal(after.ModTime()) {
			return nil, fmt.Errorf("%s changed during capture; retry on a stable working tree", name)
		}
		total += len(content)
		if total > MaxTotalBytes {
			return nil, fmt.Errorf("snapshot exceeds %d bytes", MaxTotalBytes)
		}
		s.Executable[name] = info.Mode()&0111 != 0
		if runtime.GOOS == "windows" {
			s.Executable[name] = indexExecutable[name]
		}
		s.add(name, content)
	}
	s.Digest = s.digest()
	s.Identity += ":" + s.Digest[:12]
	return s, nil
}

// A tracked child may still appear in ls-files after its parent was replaced
// by an ignored symlink. Do not let that link import hidden source into a scan.
func inspectParents(root *os.Root, name string, checked map[string]bool, omitted map[string]string) (bool, error) {
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if directory, exists := checked[parent]; exists {
			if !directory {
				return false, nil
			}
			continue
		}
		info, err := root.Lstat(parent)
		if os.IsNotExist(err) {
			checked[parent] = false
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("inspect parent %s: %w", parent, err)
		}
		checked[parent] = info.IsDir()
		if !info.IsDir() {
			omitted[parent] = "symlinks and non-directory parents are not followed"
			return false, nil
		}
	}
	return true, nil
}

func (s *Snapshot) add(name string, content []byte) {
	if bytes.HasPrefix(content, []byte("version https://git-lfs.github.com/spec/v1\n")) {
		s.Omitted[name] = "Git LFS pointer: source content is unavailable"
		return
	}
	s.Files[name] = content
}

func (s *Snapshot) digest() string {
	h := sha256.New()
	for _, name := range Keys(s.Files) {
		fmt.Fprintf(h, "%s\x00%t\x00%d\x00", name, s.Executable[name], len(s.Files[name]))
		h.Write(s.Files[name])
	}
	omitted := make([]string, 0, len(s.Omitted))
	for name := range s.Omitted {
		omitted = append(omitted, name)
	}
	sort.Strings(omitted)
	for _, name := range omitted {
		fmt.Fprintf(h, "omitted\x00%s\x00%s\x00", name, s.Omitted[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func Keys(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

// Materialize creates only regular files from a captured snapshot in an empty directory.
func (s *Snapshot) Materialize(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, name := range Keys(s.Files) {
		if !SafePath(name) {
			return fmt.Errorf("unsafe snapshot path %q", name)
		}
		if err := root.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if s.Executable[name] {
			mode = 0700
		}
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(s.Files[name])
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return err
		}
	}
	return nil
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedBuffer) String() string { return b.buffer.String() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.overflow = true
		return 0, fmt.Errorf("Git output exceeds %d bytes", b.limit)
	}
	return b.buffer.Write(p)
}

func (r *Repository) git(input []byte, limit int, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	base := []string{"--no-optional-locks", "--no-replace-objects", "-c", "core.fsmonitor=false", "-C", r.Root}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GIT_") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = bytes.NewReader(input)
	out, stderr := &boundedBuffer{limit: limit}, &boundedBuffer{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = out, stderr
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}
