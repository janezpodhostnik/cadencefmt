# Changelog

## 0.1.0

Initial release. Wraps `cadencefmt-lsp` as a VS Code formatter so users no
longer need a third-party generic LSP client extension.

- Activates on `cadence` language id.
- Format on save (via `editor.formatOnSave` + `editor.defaultFormatter`).
- `cadencefmt: Format Document with cadencefmt` command in the Command
  Palette.
- Settings: `cadencefmt.path` (binary location), `cadencefmt.trace.server`
  (LSP trace verbosity), and `cadencefmt.checkForUpdates` (GitHub Releases
  poll, default on).
- Distributed via [GitHub Releases](https://github.com/janezpodhostnik/cadencefmt/releases),
  not the VS Code Marketplace. Install via the repo's
  [one-line installer](https://github.com/janezpodhostnik/cadencefmt#one-line-installer-macos--linux)
  or by downloading the `.vsix` from the release page.
