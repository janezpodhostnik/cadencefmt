# cadencefmt

VS Code extension that formats [Cadence](https://cadence-lang.org/) source
files using [`cadencefmt`](https://github.com/janezpodhostnik/cadencefmt) —
a deterministic, idempotent formatter that preserves comments and verifies
output via round-trip AST comparison.

## Install

The recommended path is the **one-line installer** (macOS / Linux), which
fetches the binary and the extension from the latest GitHub Release:

```bash
curl -fsSL https://raw.githubusercontent.com/janezpodhostnik/cadencefmt/main/install.sh | bash
```

The script auto-detects `code` / `cursor` / `codium` / `code-insiders` and
installs into the first one it finds. Re-run to upgrade. Windows is not
currently supported.

### Manual install

Download the `.vsix` and the right `cadencefmt-lsp-<os>-<arch>` binary
from the latest [GitHub Release](https://github.com/janezpodhostnik/cadencefmt/releases/latest).
Drop the binary on your PATH, then:

```bash
code --install-extension cadencefmt.vsix
```

### Prerequisites

- The official [Cadence](https://marketplace.visualstudio.com/items?itemName=onflow.cadence)
  extension (`onflow.cadence`) — registers the `cadence` language id this
  extension binds to.

## Usage

Open a `.cdc` file. With format-on-save enabled this extension formats on
every save:

```json
{
  "[cadence]": {
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "janezpodhostnik.cadencefmt"
  }
}
```

Or run **cadencefmt: Format Document with cadencefmt** from the Command
Palette.

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `cadencefmt.path` | `cadencefmt-lsp` | Path or command for the `cadencefmt-lsp` binary. |
| `cadencefmt.trace.server` | `off` | LSP traffic verbosity: `off`, `messages`, or `verbose`. Output appears in the **cadencefmt** output channel. |
| `cadencefmt.checkForUpdates` | `true` | Check GitHub Releases at most once per day for a new version and show a one-time notification. Set to `false` to disable. |

## Troubleshooting

- **Nothing happens on save.** Verify `editor.defaultFormatter` for
  `[cadence]` is set to `janezpodhostnik.cadencefmt`.
- **"failed to start language server".** Run `cadencefmt-lsp --version`
  in a terminal. If that works but VS Code can't find the binary, your
  shell PATH and VS Code's PATH differ — set an absolute path in
  `cadencefmt.path`.
- **Formatter runs but the file doesn't change.** Set
  `cadencefmt.trace.server` to `verbose`, reload the window, and check
  the **cadencefmt** output channel. Parse errors cause the server to
  return no edits silently to avoid disrupting the editor; run
  `cadencefmt < file.cdc` from a terminal to see the actual error.

## Source

[github.com/janezpodhostnik/cadencefmt](https://github.com/janezpodhostnik/cadencefmt)
— issues, PRs, and changelog. The extension itself lives at
[`editors/vscode/`](https://github.com/janezpodhostnik/cadencefmt/tree/main/editors/vscode).

## License

[Apache-2.0](LICENSE)
