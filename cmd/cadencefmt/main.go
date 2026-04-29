package main

import (
	"fmt"
	"io"
	"os"

	"github.com/janezpodhostnik/cadencefmt/internal/format"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Println("cadencefmt " + version)
			os.Exit(0)
		case "--help", "-h":
			printHelp()
			os.Exit(0)
		}
	}

	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(2)
	}

	if len(src) == 0 {
		printHelp()
		os.Exit(2)
	}

	out, err := format.Format(src, "<stdin>", format.Default())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}

	os.Stdout.Write(out)
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `cadencefmt — formatter for the Cadence smart contract language

Usage:
  cadencefmt [flags] [path...]

When no paths are given, reads from stdin and writes to stdout.

Flags:
  -w, --write          Write changes back to source files
  -c, --check          Exit 1 if any input would change
  -d, --diff           Print unified diff of changes
      --config FILE    Use this config file
      --no-config      Ignore config files; use defaults
      --stdin-filename Filename for diagnostics when reading stdin
      --no-verify      Skip round-trip AST equivalence check
      --version        Print version and exit
  -h, --help           Print this help and exit
`)
}
