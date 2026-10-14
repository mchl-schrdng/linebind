// Command smoke exercises an installed binary from a temporary repository.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	binary := flag.String("binary", "", "path to the installed or extracted Linebind binary")
	version := flag.String("version", "", "expected version in --version output")
	flag.Parse()
	if err := smoke(*binary, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func smoke(binary, version string) error {
	if binary == "" {
		return fmt.Errorf("-binary is required")
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(binary, ".exe") {
		binary += ".exe"
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "linebind smoke ")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	commands := [][]string{{"git", "init", "--quiet"}, {binary, "--version"}, {binary, "--help"}, {binary, "init"}, {binary, "validate"}}
	for _, args := range commands {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, output)
		}
		if args[1] == "--version" && version != "" && !strings.Contains(string(output), version) {
			return fmt.Errorf("expected version %q, received %q", version, output)
		}
		if args[1] == "--help" && (!strings.Contains(string(output), "init") || !strings.Contains(string(output), "validate")) {
			return fmt.Errorf("help is missing init or validate: %s", output)
		}
		fmt.Printf("PASS %s\n", strings.Join(args[1:], " "))
	}
	return nil
}
