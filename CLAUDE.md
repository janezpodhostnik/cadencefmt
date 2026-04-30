# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

cadencefmt is a deterministic, idempotent formatter for the Cadence smart contract language (Flow blockchain). It produces two binaries: `cadencefmt` (CLI, stdin/stdout filter) and `cadencefmt-lsp` (LSP server for editor integration). Written in Go, no CGO.

Module path: `github.com/janezpodhostnik/cadencefmt`

## Architecture

7-stage pipeline: parse -> scan comments -> attach comments -> rewrite AST -> render to Doc IR -> pretty-print -> verify round-trip.

- **`internal/format/trivia/`** - Novel comment extraction. Hand-written lexer scans source bytes for comments (the `onflow/cadence` parser doesn't retain them in the AST). Attaches comments to AST nodes by position, producing a `CommentMap`. This is the most complex module.
- **`internal/format/rewrite/`** - Sequential AST mutation passes (imports sorting, modifier ordering, redundant paren removal). Fixed order matters for idempotence.
- **`internal/format/render/`** - Converts AST + CommentMap into `prettier.Doc` IR. Delegates to existing `ast.Element.Doc()` methods where possible, overrides for custom style rules. Comments interleaved via `CommentMap.Take()`.
- **`internal/format/verify/`** - Re-parses formatted output and structurally compares ASTs. Safety net for correctness.
- **`internal/lsp/`** - LSP server, `textDocument/formatting` only.
- **`internal/diff/`** - Unified diff for `--check`/`--diff` output.

Pipeline entry point: `format.Format()` in `internal/format/formatter.go` orchestrates all stages.

Key dependencies: `github.com/onflow/cadence` for parser/AST, `github.com/turbolent/prettier` for Wadler-style pretty-printing IR, `github.com/spf13/cobra` for CLI.

## Hard Invariants

These must never be violated:
1. **Round-trip correctness**: `parse(format(S))` structurally equals `parse(S)`
2. **Idempotence**: `format(format(S)) == format(S)` byte-for-byte
3. **Comment preservation**: every comment appears exactly once, same logical position
4. **Fail-safe**: parse errors -> exit non-zero, nothing written to stdout

## Build & Development

```bash
direnv allow                         # auto-loads nix dev shell (or: nix develop)
just build                           # build both binaries
just test                            # run all tests (fast, excludes corpus)
just corpus                          # run corpus tests (requires submodule)
just lint                            # golangci-lint
just fuzz                            # fuzz for 60s per target
just update-golden                   # refresh golden files
just snapshot <name>                 # run a single snapshot test
just check                           # build + test + lint
```

## Testing

- **Snapshot tests**: `testdata/format/<case>/input.cdc` + `golden.cdc`. Use `-update` flag to refresh goldens.
- **Idempotence tests**: `format(format(input)) == format(input)` for every snapshot case.
- **Round-trip AST tests**: parse both input and output, structurally compare.
- **Comment preservation**: multiset equality of comments between input and output.
- **Fuzzing**: `FuzzFormat` (no panics on arbitrary bytes) and `FuzzRoundtrip` (idempotence + AST on valid inputs).
- **Corpus tests**: `testdata/corpus/` contains real-world Flow contracts via git submodules (`flow-core-contracts`, `flow-ft`, `flow-nft`). `TestCorpus` checks format, idempotence, round-trip, and comment preservation. Skipped with `-short`. Run with `just corpus`.

## CLI Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | `--check`: at least one file would change |
| 2 | Usage error (bad flags, missing input) |
| 3 | Parse error in input |
| 4 | Internal error (verify failed, orphaned comments) |

## Default Formatting Options

Line width: 100, indent: 4 spaces (no tabs), sort imports: yes, strip semicolons: yes, keep at most 1 blank line. Configured via `format.Options` in `internal/format/options.go`.

## CI

GitHub Actions (`.github/workflows/ci.yml`): uses the Nix flake for environment setup (single source of truth for Go version and tools). Runs build, tests, corpus tests (`continue-on-error`), and fuzz on ubuntu-latest. Submodules checked out automatically.

## Key Design Decisions

- Comments extracted out-of-band (not from AST) because `onflow/cadence` parser doesn't retain them (`onflow/cadence#308`). Approach mirrors `go/ast.CommentMap`.
- `CommentMap.Take()` removes comments on access -- renderer asserts map is empty after rendering to catch orphaned comments.
- Deprecated `pub`/`priv` modifiers are preserved as-written, not rewritten.
- String template interpolations `\(expr)` are not reformatted in v1.
- All `internal/` packages are private by design. Public surface is CLI + LSP only.
- Rewriter pass order is fixed and must not be reordered without bumping `format_version`.
- Do not fork or modify the `onflow/cadence` parser. Use it as a library only.
- Do not add new IR primitives to `turbolent/prettier`. Use the existing algebra.

## Comment Scanner Edge Cases

The trivia scanner is the trickiest module. Key gotchas:
- **Nested block comments**: Cadence supports `/* /* */ */`. Track nesting depth.
- **Strings**: Skip comment-like sequences inside string literals (`"// not a comment"`). Handle `\"` escapes.
- **String templates**: `\(expr)` requires nested paren counting to find the closing `)` before resuming string state.
- **Doc-line vs regular**: `///` is doc-line only if the 4th char is not `/`. `////` is a regular line comment.
- **Doc-block vs regular**: `/**` is doc-block only if the 4th char is not `*` and the comment is not `/**/`.

## Debugging Tips

- **"orphaned comments" error**: CommentMap has comments no render function called `Take()` for. Use the positions in the error to find which AST node type is missing a `wrapWithComments` call.
- **Idempotence failure**: Format the output a second time and diff. The difference shows which construct isn't stable. Often caused by trailing whitespace, inconsistent blank line handling, or a Group that breaks differently on re-format.
- **Round-trip failure**: AST of formatted output doesn't match the original. Diff the AST dumps. Usually means the renderer emits something the parser interprets differently (e.g., operator precedence changes when parens are removed).
- **Comment in wrong position**: Print the CommentMap after attachment. Check the comment's source position falls within the expected node's range. The disambiguation heuristic in `trivia/attach.go` (which decides between trailing-of-previous vs leading-of-next when a comment sits between two nodes) is usually the culprit.
- **Exploring the cadence AST**: Use `go doc github.com/onflow/cadence/ast`. The `ast.Walk` function and `ast.Element` interface are the main tools.
