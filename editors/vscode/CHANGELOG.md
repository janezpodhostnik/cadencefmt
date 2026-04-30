# Changelog

## 0.1.0

Initial release. Wraps `cadencefmt-lsp` as a VS Code formatter so users no
longer need a third-party generic LSP client extension.

- Activates on `cadence` language id.
- Format on save (via `editor.formatOnSave` + `editor.defaultFormatter`).
- `cadencefmt: Format Document with cadencefmt` command in the Command
  Palette.
- Settings: `cadencefmt.path` (binary location) and `cadencefmt.trace.server`
  (LSP trace verbosity).
