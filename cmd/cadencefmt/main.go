package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/janezpodhostnik/cadencefmt/internal/config"
	"github.com/janezpodhostnik/cadencefmt/internal/diff"
	"github.com/janezpodhostnik/cadencefmt/internal/format"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	flagWrite         bool
	flagCheck         bool
	flagDiff          bool
	flagNoVerify      bool
	flagStdinFilename string
	flagConfig        string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cadencefmt [flags] [path...]",
		Short: "Formatter for the Cadence smart contract language",
		Long:  "cadencefmt — deterministic, idempotent formatter for Cadence (.cdc) source files.",
		RunE:  run,
		Args:  cobra.ArbitraryArgs,
		// Silence default usage/error printing — we handle exit codes manually.
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	flags := rootCmd.Flags()
	flags.BoolVarP(&flagWrite, "write", "w", false, "Write changes back to source files")
	flags.BoolVarP(&flagCheck, "check", "c", false, "Exit 1 if any input would change")
	flags.BoolVarP(&flagDiff, "diff", "d", false, "Print unified diff of changes")
	flags.BoolVar(&flagNoVerify, "no-verify", false, "Skip round-trip AST equivalence check")
	flags.StringVar(&flagStdinFilename, "stdin-filename", "", "Filename for diagnostics when reading stdin")
	flags.StringVar(&flagConfig, "config", "", "Path to config file (default: search for .cadencefmt.toml)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if err := validateFlags(len(args) == 0); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	// No paths: read from stdin
	if len(args) == 0 {
		return formatStdin()
	}

	// With paths: process files
	exitCode := 0
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			exitCode = 2
			continue
		}

		if info.IsDir() {
			code := walkDir(path)
			if code > exitCode {
				exitCode = code
			}
		} else {
			code := formatFile(path)
			if code > exitCode {
				exitCode = code
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// validateFlags rejects mutually-exclusive flag combinations that would
// otherwise produce surprising silent precedence behavior.
func validateFlags(stdin bool) error {
	if flagCheck && flagWrite {
		return fmt.Errorf("flags --check and --write are mutually exclusive")
	}
	if flagCheck && flagDiff {
		return fmt.Errorf("flags --check and --diff are mutually exclusive")
	}
	if flagWrite && flagDiff {
		return fmt.Errorf("flags --write and --diff are mutually exclusive")
	}
	if stdin && flagWrite {
		return fmt.Errorf("--write requires file paths; cannot be used with stdin input")
	}
	return nil
}

func formatOpts(basePath string) format.Options {
	opts := format.Default()

	// Load config: explicit --config flag or walk-up search
	var cfg config.Config
	if flagConfig != "" {
		c, err := config.ParseFile(flagConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		cfg = c
	} else if basePath != "" {
		c, _, err := config.Lookup(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(2)
		}
		cfg = c
	}

	opts = cfg.Apply(opts)

	// CLI flags override config
	opts.SkipVerify = flagNoVerify
	return opts
}

func formatStdin() error {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(2)
	}

	if len(src) == 0 {
		return fmt.Errorf("no input")
	}

	filename := "<stdin>"
	if flagStdinFilename != "" {
		filename = flagStdinFilename
	}

	out, err := format.Format(src, filename, formatOpts("."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if errors.Is(err, format.ErrInternal) {
			os.Exit(4)
		}
		os.Exit(3)
	}

	if flagCheck {
		if !bytes.Equal(src, out) {
			fmt.Fprintln(os.Stderr, filename)
			os.Exit(1)
		}
		return nil
	}

	if flagDiff {
		d := diff.Unified(filename, string(src), string(out))
		if d != "" {
			fmt.Print(d)
		}
		return nil
	}

	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(2)
	}
	return nil
}

func formatFile(path string) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
		return 2
	}

	out, err := format.Format(src, path, formatOpts(filepath.Dir(path)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
		if errors.Is(err, format.ErrInternal) {
			return 4
		}
		return 3
	}

	if bytes.Equal(src, out) {
		return 0 // no changes
	}

	if flagCheck {
		fmt.Println(path)
		return 1
	}

	if flagDiff {
		d := diff.Unified(path, string(src), string(out))
		if d != "" {
			fmt.Print(d)
		}
		return 0
	}

	if flagWrite {
		if err := os.WriteFile(path, out, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
			return 2
		}
		return 0
	}

	// Without -w, print formatted output to stdout
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
		return 2
	}
	return 0
}

func walkDir(root string) int {
	exitCode := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			exitCode = 2
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".cdc") {
			return nil
		}
		// Don't follow symlinks
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		code := formatFile(path)
		if code > exitCode {
			exitCode = code
		}
		return nil
	})
	return exitCode
}
