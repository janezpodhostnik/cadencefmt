package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cadencefmt-test")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	binaryPath = filepath.Join(tmp, "cadencefmt")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("failed to build: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func runCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	exitCode = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func TestCLI_Stdin(t *testing.T) {
	t.Parallel()
	input := "access(all) fun   main()  {  }\n"
	stdout, _, code := runCLI(t, input)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "access(all) fun main() {}") {
		t.Errorf("unexpected output:\n%s", stdout)
	}
}

func TestCLI_Write(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.cdc")
	if err := os.WriteFile(path, []byte("access(all) fun   main()  {  }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runCLI(t, "", "-w", path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "access(all) fun main() {}") {
		t.Errorf("file not formatted:\n%s", got)
	}
}

func TestCLI_Check_Clean(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.cdc")
	if err := os.WriteFile(path, []byte("access(all) fun main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runCLI(t, "", "-c", path)
	if code != 0 {
		t.Fatalf("expected exit 0 for clean file, got %d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected no stdout for clean file, got: %q", stdout)
	}
}

func TestCLI_Check_Dirty(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.cdc")
	if err := os.WriteFile(path, []byte("access(all) fun   main()  {  }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runCLI(t, "", "-c", path)
	if code != 1 {
		t.Fatalf("expected exit 1 for dirty file, got %d", code)
	}
	if !strings.Contains(stdout, "test.cdc") {
		t.Errorf("expected filename in output, got: %q", stdout)
	}
}

func TestCLI_Diff(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.cdc")
	if err := os.WriteFile(path, []byte("access(all) fun   main()  {  }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runCLI(t, "", "-d", path)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "---") || !strings.Contains(stdout, "+++") {
		t.Errorf("expected unified diff markers, got:\n%s", stdout)
	}
}

func TestCLI_StdinFilename(t *testing.T) {
	t.Parallel()
	// --stdin-filename is used in --check output and --diff header for stdin
	unformatted := "access(all) fun   main()  {  }\n"
	_, stderr, code := runCLI(t, unformatted, "--check", "--stdin-filename", "foo.cdc")
	if code != 1 {
		t.Fatalf("expected exit 1 for dirty stdin, got %d", code)
	}
	if !strings.Contains(stderr, "foo.cdc") {
		t.Errorf("expected filename in check output, got stderr=%q", stderr)
	}
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()
	stdout, _, code := runCLI(t, "", "--version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "cadencefmt version") {
		t.Errorf("expected version output, got: %q", stdout)
	}
}

func TestCLI_NoVerify(t *testing.T) {
	t.Parallel()
	stdout, _, code := runCLI(t, "access(all) fun main() {}\n", "--no-verify")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "access(all) fun main() {}") {
		t.Errorf("unexpected output:\n%s", stdout)
	}
}

func TestCLI_ParseError(t *testing.T) {
	t.Parallel()
	_, _, code := runCLI(t, "this is not valid cadence {{{{")
	if code != 3 {
		t.Fatalf("expected exit 3 for parse error, got %d", code)
	}
}

func TestCLI_Directory(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	unformatted := "access(all) fun   main()  {  }\n"
	for _, name := range []string{"a.cdc", "b.cdc"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(unformatted), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Non-.cdc file should be ignored
	if err := os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runCLI(t, "", "-w", tmp)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	for _, name := range []string{"a.cdc", "b.cdc"} {
		got, _ := os.ReadFile(filepath.Join(tmp, name))
		if !strings.Contains(string(got), "access(all) fun main() {}") {
			t.Errorf("%s not formatted:\n%s", name, got)
		}
	}

	// Non-.cdc file should be untouched
	txt, _ := os.ReadFile(filepath.Join(tmp, "readme.txt"))
	if string(txt) != "hello" {
		t.Errorf("non-.cdc file was modified: %q", txt)
	}
}

// --- Config file tests ---

func TestCLI_ConfigFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create config with 2-space indent
	cfgContent := "indent_count = 2\n"
	if err := os.WriteFile(filepath.Join(tmp, ".cadencefmt.toml"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a .cdc file with nested code
	src := "access(all) fun main() {\nlet x = 1\n}\n"
	cdcPath := filepath.Join(tmp, "test.cdc")
	if err := os.WriteFile(cdcPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runCLI(t, "", cdcPath)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Should use 2-space indent from config
	if !strings.Contains(stdout, "\n  let x") {
		t.Errorf("expected 2-space indent from config, got:\n%s", stdout)
	}
}

func TestCLI_ConfigFlag(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Config in a non-standard location
	cfgPath := filepath.Join(tmp, "custom-config.toml")
	if err := os.WriteFile(cfgPath, []byte("indent_count = 3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	src := "access(all) fun main() {\nlet x = 1\n}\n"
	stdout, _, code := runCLI(t, src, "--config", cfgPath)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "\n   let x") {
		t.Errorf("expected 3-space indent from --config flag, got:\n%s", stdout)
	}
}

func TestCLI_ConfigWalkUp(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Config in parent dir
	if err := os.WriteFile(filepath.Join(tmp, ".cadencefmt.toml"), []byte("indent_count = 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// File in child dir
	child := filepath.Join(tmp, "contracts")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	cdcPath := filepath.Join(child, "test.cdc")
	if err := os.WriteFile(cdcPath, []byte("access(all) fun main() {\nlet x = 1\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runCLI(t, "", cdcPath)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "\n  let x") {
		t.Errorf("expected 2-space indent from parent config, got:\n%s", stdout)
	}
}
