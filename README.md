# cadencefmt

Deterministic, idempotent formatter for the [Cadence](https://cadence-lang.org/) smart contract language.

- Preserves all comments in their logical position
- Sorts imports alphabetically
- Verifies correctness via round-trip AST comparison
- Ships as a CLI (`cadencefmt`) and LSP server (`cadencefmt-lsp`)

## Status

> [!WARNING]
> Early version — formatting works on the corpus we've tested, but you will likely
> hit bugs on real-world code. Please [open an issue](https://github.com/janezpodhostnik/cadencefmt/issues)
> with a minimal reproduction (input `.cdc` plus expected vs actual output).

> [!NOTE]
> Most of this codebase was written with AI assistance (Claude). Every change is
> reviewed, tested against snapshot/corpus/round-trip suites, and gated on the
> hard invariants documented in [CLAUDE.md](CLAUDE.md).

## Example

<table>
<tr><th>Before</th><th>After</th></tr>
<tr>
<td><img src="doc/assets/before.png" alt="Unformatted Cadence source" /></td>
<td><img src="doc/assets/after.png" alt="Formatted Cadence source" /></td>
</tr>
</table>

## Installation

### One-line installer (macOS / Linux)

Installs the `cadencefmt` and `cadencefmt-lsp` binaries plus the VS Code
extension. Re-run to upgrade.

```bash
curl -fsSL https://raw.githubusercontent.com/janezpodhostnik/cadencefmt/main/install.sh | bash
```

The script downloads from the latest [GitHub Release](https://github.com/janezpodhostnik/cadencefmt/releases/latest)
and detects `code` / `cursor` / `codium` / `code-insiders` automatically.
Set `PREFIX=/some/dir` to control where binaries land (default `~/.local/bin`).

### Manual install

Pre-built binaries (macOS and Linux on amd64 and arm64) and the VS Code
extension `.vsix` are attached to every [GitHub Release](https://github.com/janezpodhostnik/cadencefmt/releases/latest).
Pick the right binary for your platform, drop it on `PATH`, then install
the extension via `code --install-extension cadencefmt.vsix`.

### Build from source

```bash
# Go
go install github.com/janezpodhostnik/cadencefmt/cmd/cadencefmt@latest
go install github.com/janezpodhostnik/cadencefmt/cmd/cadencefmt-lsp@latest

# Nix
nix run github:janezpodhostnik/cadencefmt

# Clone + build
git clone https://github.com/janezpodhostnik/cadencefmt.git
cd cadencefmt
go build ./cmd/cadencefmt
go build ./cmd/cadencefmt-lsp
```

### Windows

Windows is not currently supported — no pre-built binary, no installer
script. Building from source via `go build` should work, but the binaries
are untested on Windows. Issues / PRs welcome.

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

# Separate flags from paths starting with -
cadencefmt -w -- -unusual-name.cdc
```

### Flags

| Flag | Description |
|------|-------------|
| `-w`, `--write` | Write formatted output back to the file |
| `-c`, `--check` | Exit 1 if any file would change; prints changed paths |
| `-d`, `--diff` | Print unified diff instead of formatted source |
| `--no-verify` | Skip round-trip AST verification |
| `--config` | Path to config file (overrides `.cadencefmt.toml` search) |
| `--stdin-filename` | Filename for diagnostics when reading stdin |
| `-v`, `--version` | Print version and exit |

### LSP Server

`cadencefmt-lsp` speaks [LSP](https://microsoft.github.io/language-server-protocol/) over stdio and supports `textDocument/formatting`.

For **VS Code** (and forks like Cursor / VSCodium), use the [one-line installer](#one-line-installer-macos--linux). For other editors, point any generic LSP client at the binary.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | `--check`: at least one file would change |
| 2 | Usage error (bad flags, missing input) |
| 3 | Parse error in input |
| 4 | Internal error (verification failed, orphaned comments) |

## Formatting Style

Defaults (configurable via `.cadencefmt.toml`):

- 100-character line width (`line_width`, default: 100)
- 4-space indentation (`indent_character`: `" "`, `indent_count`: 4; tabs supported)
- Sorted imports (`sort_imports`, default: true)
- Stripped semicolons (`strip_semicolons`, default: true)
- At most 1 consecutive blank line (`keep_blank_lines`, default: 1)

## Configuration

Create a `.cadencefmt.toml` in your project root:

```toml
line_width = 80
indent_character = " "
indent_count = 2
sort_imports = true
strip_semicolons = true
keep_blank_lines = 1
```

The formatter searches for `.cadencefmt.toml` starting from the formatted file's directory, walking up to the filesystem root. All fields are optional — unset fields use defaults.

Use `--config <path>` to specify an explicit config file.

## Performance

Representative benchmark results formatting real-world Cadence contracts:

| Input | Time | Throughput | Allocs |
|-------|-----:|-----------:|-------:|
| Snapshots (30 files, 4.8KB total) | 1.7ms | 2.9 MB/s | 20K |
| Corpus small (<1KB, 183 files) | 42ms | 3.3 MB/s | 536K |
| Corpus medium (1–10KB, 257 files) | 114ms | 3.9 MB/s | 1.4M |
| Corpus large (>10KB, 46 files) | 219ms | 3.9 MB/s | 2.2M |
| Largest file (110KB) | 35ms | 3.1 MB/s | 312K |

> Measured on AMD Ryzen 9 3900X, Go 1.26.1, Linux. Numbers vary by hardware.
> Reproduce: `just bench-all` (requires `git submodule update --init` for corpus).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and commit conventions.

## License

[Apache-2.0](LICENSE)
