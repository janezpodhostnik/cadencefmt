package main

import (
	"context"
	"fmt"
	"os"

	"github.com/janezpodhostnik/cadencefmt/internal/lsp"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Println("cadencefmt-lsp " + version)
			os.Exit(0)
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "cadencefmt-lsp — LSP server for Cadence formatting")
			fmt.Fprintln(os.Stderr, "Speaks LSP over stdio. Supports textDocument/formatting only.")
			fmt.Fprintln(os.Stderr, "\nUsage: cadencefmt-lsp")
			os.Exit(0)
		}
	}

	srv := lsp.NewServer()
	if err := srv.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "lsp server error: %v\n", err)
		os.Exit(1)
	}
}
