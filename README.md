# cadencefmt

Deterministic, idempotent formatter for the [Cadence](https://cadence-lang.org/) smart contract language.

- Preserves all comments in their logical position
- Sorts imports alphabetically
- Verifies correctness via round-trip AST comparison
- Ships as a CLI (`cadencefmt`) and LSP server (`cadencefmt-lsp`)

## Installation

```bash
# Go
go install github.com/janezpodhostnik/cadencefmt/cmd/cadencefmt@latest
go install github.com/janezpodhostnik/cadencefmt/cmd/cadencefmt-lsp@latest

# Nix
nix run github:janezpodhostnik/cadencefmt

# Build from source
git clone https://github.com/janezpodhostnik/cadencefmt.git
cd cadencefmt
go build ./cmd/cadencefmt
go build ./cmd/cadencefmt-lsp
```

## Usage

```bash
# Format stdin → stdout
cat MyContract.cdc | cadencefmt

# Format files in-place
cadencefmt -w MyContract.cdc

# Format a directory recursively
cadencefmt -w contracts/

# Check if files are formatted (exit 1 if not)
cadencefmt -c contracts/

# Show diff of formatting changes
cadencefmt -d MyContract.cdc
```

### Flags

| Flag | Description |
|------|-------------|
| `-w`, `--write` | Write formatted output back to the file |
| `-c`, `--check` | Exit 1 if any file would change (no output) |
| `-d`, `--diff` | Print unified diff instead of formatted source |
| `--no-verify` | Skip round-trip AST verification |
| `--stdin-filename` | Filename for diagnostics when reading stdin |

### LSP Server

`cadencefmt-lsp` speaks [LSP](https://microsoft.github.io/language-server-protocol/) over stdio and supports `textDocument/formatting`. Point your editor's generic LSP client at it.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | `--check`: at least one file would change |
| 2 | Usage error (bad flags, missing input) |
| 3 | Parse error in input |
| 4 | Internal error (verification failed, orphaned comments) |

## Formatting Style

Defaults (configurable via the Go API `format.Options`):

- 100-character line width
- 4-space indentation (no tabs)
- Sorted imports
- Stripped semicolons (`StripSemicolons`, default: true)
- At most 1 consecutive blank line (`KeepBlankLines`, default: 1)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and commit conventions.

## License

[Apache-2.0](LICENSE)
