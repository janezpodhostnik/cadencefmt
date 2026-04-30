package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janezpodhostnik/cadencefmt/internal/format"
)

func TestLookup_Found(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, FileName)
	if err := os.WriteFile(cfgPath, []byte("line_width = 80\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Lookup(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if path != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, path)
	}
	if cfg.LineWidth == nil || *cfg.LineWidth != 80 {
		t.Errorf("expected LineWidth=80, got %v", cfg.LineWidth)
	}
}

func TestLookup_WalkUp(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	child := filepath.Join(tmp, "sub", "dir")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(tmp, FileName)
	if err := os.WriteFile(cfgPath, []byte("indent_count = 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Lookup(child)
	if err != nil {
		t.Fatal(err)
	}
	if path != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, path)
	}
	if cfg.IndentCount == nil || *cfg.IndentCount != 2 {
		t.Errorf("expected IndentCount=2, got %v", cfg.IndentCount)
	}
}

func TestLookup_NotFound(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	child := filepath.Join(tmp, "empty")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := Lookup(child)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("expected empty path, got %s", path)
	}
	if cfg.LineWidth != nil {
		t.Error("expected nil LineWidth for missing config")
	}
}

func TestApply_Partial(t *testing.T) {
	t.Parallel()
	lw := 80
	cfg := Config{LineWidth: &lw}
	opts := cfg.Apply(format.Default())

	if opts.LineWidth != 80 {
		t.Errorf("expected LineWidth=80, got %d", opts.LineWidth)
	}
	// Other fields should keep defaults
	if opts.IndentCount != 4 {
		t.Errorf("expected IndentCount=4 (default), got %d", opts.IndentCount)
	}
	if opts.IndentCharacter != " " {
		t.Errorf("expected IndentCharacter=space (default), got %q", opts.IndentCharacter)
	}
}

func TestApply_Full(t *testing.T) {
	t.Parallel()
	lw := 80
	ic := "\t"
	icn := 1
	si := false
	ss := false
	kb := 2
	cfg := Config{
		LineWidth:       &lw,
		IndentCharacter: &ic,
		IndentCount:     &icn,
		SortImports:     &si,
		StripSemicolons: &ss,
		KeepBlankLines:  &kb,
	}
	opts := cfg.Apply(format.Default())

	if opts.LineWidth != 80 {
		t.Errorf("LineWidth: got %d", opts.LineWidth)
	}
	if opts.IndentCharacter != "\t" {
		t.Errorf("IndentCharacter: got %q", opts.IndentCharacter)
	}
	if opts.IndentCount != 1 {
		t.Errorf("IndentCount: got %d", opts.IndentCount)
	}
	if opts.SortImports != false {
		t.Error("SortImports: expected false")
	}
	if opts.StripSemicolons != false {
		t.Error("StripSemicolons: expected false")
	}
	if opts.KeepBlankLines != 2 {
		t.Errorf("KeepBlankLines: got %d", opts.KeepBlankLines)
	}
}

func TestApply_Empty(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	defaults := format.Default()
	opts := cfg.Apply(defaults)

	if opts != defaults {
		t.Errorf("empty config should not change defaults:\ngot:  %+v\nwant: %+v", opts, defaults)
	}
}

func TestParse_Valid(t *testing.T) {
	t.Parallel()
	data := []byte(`
line_width = 80
indent_character = "\t"
indent_count = 1
sort_imports = false
strip_semicolons = false
keep_blank_lines = 2
`)
	cfg, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LineWidth == nil || *cfg.LineWidth != 80 {
		t.Errorf("LineWidth: got %v", cfg.LineWidth)
	}
	if cfg.IndentCharacter == nil || *cfg.IndentCharacter != "\t" {
		t.Errorf("IndentCharacter: got %v", cfg.IndentCharacter)
	}
	if cfg.SortImports == nil || *cfg.SortImports != false {
		t.Errorf("SortImports: got %v", cfg.SortImports)
	}
}

func TestParse_Invalid(t *testing.T) {
	t.Parallel()
	_, err := Parse([]byte("this is not valid toml [[["))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestParse_PartialFields(t *testing.T) {
	t.Parallel()
	cfg, err := Parse([]byte("line_width = 120\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LineWidth == nil || *cfg.LineWidth != 120 {
		t.Errorf("LineWidth: got %v", cfg.LineWidth)
	}
	// All other fields should be nil
	if cfg.IndentCharacter != nil {
		t.Error("IndentCharacter should be nil")
	}
	if cfg.IndentCount != nil {
		t.Error("IndentCount should be nil")
	}
}
