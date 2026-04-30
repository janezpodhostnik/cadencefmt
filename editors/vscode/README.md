# cadencefmt

VS Code extension that formats [Cadence](https://cadence-lang.org/) source
files using [`cadencefmt`](https://github.com/janezpodhostnik/cadencefmt) —
a deterministic, idempotent formatter that preserves comments and verifies
output via round-trip AST comparison.

## Prerequisites

- The official **Cadence** extension
  ([`onflow.cadence`](https://marketplace.visualstudio.com/items?itemName=onflow.cadence))
  for syntax highlighting and the `cadence` language id this extension
  binds to.
- The `cadencefmt-lsp` binary on your PATH, installed via Go:

  ```bash
  go install github.com/janezpodhostnik/cadencefmt/cmd/cadencefmt-lsp@latest
  ```

  This places the binary in `$(go env GOPATH)/bin` (typically `~/go/bin`
  on macOS/Linux, `%USERPROFILE%\go\bin` on Windows). Make sure that
  directory is on your PATH, or set `cadencefmt.path` (see Settings below).

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
