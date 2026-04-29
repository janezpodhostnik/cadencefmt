# Cadence Formatter — Technical Specification

**Document status:** v1 design spec, intended as implementation reference for an AI agent or human contributor building the formatter from scratch in a fresh repository.

**Working name:** `cadencefmt` (matching the `gofmt`/`rustfmt` convention; final name TBD).

---

## 1. Goals & non-goals

### 1.1 Goals
- Produce a deterministic, idempotent canonical formatting of Cadence (`.cdc`) source files.
- Preserve all comments and their semantic association with surrounding code.
- Distribute as a single static Go binary, drop-in usable from any editor that can shell out to a stdin/stdout filter.
- Implement an LSP server exposing only `textDocument/formatting` (and later `textDocument/rangeFormatting`) so any LSP-capable editor (Neovim, Helix, VSCode via a generic LSP client) can consume it without a bespoke per-editor extension.
- Architecturally model after `apple/swift-format`: a multi-pass pipeline of AST rewrites followed by a Wadler/Oppen-style pretty printer.
- Build on the canonical `onflow/cadence` Go parser. Do not maintain a separate parser.

### 1.2 Non-goals (v1)
- Linting beyond what's needed to drive formatting decisions (a separate `cadence-lint` already exists upstream; we won't compete with it).
- Range formatting on first release (added in v1.1 once full-document formatting is stable).
- Auto-fixing semantic issues (e.g. updating deprecated `pub`/`priv` to `access(all)`/`access(self)`); explicit migrations are out of scope.
- WebAssembly distribution. The Go binary covers all editor use cases via stdin/stdout. A WASM build can come later for browser playgrounds.
- Formatting Cadence embedded in `.cdc.json` arguments or in JS/TS template strings. Only `.cdc` files.
- A dedicated VSCode extension. VSCode users are supported via the LSP server (consumed by a generic LSP client extension) or by piping through the CLI from any "format on save" extension. A bespoke extension is deferred until after v1 ships and the formatter has been validated against the corpus.

### 1.3 Hard invariants
1. **Round-trip correctness:** for any input `S` that parses without errors, `parse(format(S))` must produce a structurally equivalent AST to `parse(S)`. Whitespace, comment placement, and trivial syntactic sugar may change; semantics never may.
2. **Idempotence:** `format(format(S)) == format(S)`, byte-for-byte.
3. **Comment preservation:** every comment in `S` appears exactly once in `format(S)`, attached to the same logical position. No comment is ever silently dropped.
4. **Failure mode:** if input fails to parse, the formatter exits non-zero, prints the parser's error to stderr, and writes nothing to stdout (does not write a partially formatted file).

---

## 2. Background & rationale

The Cadence team has tracked this work as [`onflow/cadence#209`](https://github.com/onflow/cadence/issues/209) since 2020. The state at time of writing:

- Every AST element (`ast.Element`) implements a `Doc() prettier.Doc` method (completed in PR #1520).
- The pretty-printing library `github.com/turbolent/prettier` (MIT, by the Cadence lead) is a faithful implementation of Wadler 2003.
- Position information is fully populated on AST nodes (issue #210, completed).
- The single remaining blocker for the in-tree pretty printer is comment retention in the AST ([`onflow/cadence#308`](https://github.com/onflow/cadence/issues/308)).

This formatter avoids waiting for upstream comment retention by extracting comments **out-of-band** from the source bytes and attaching them to AST positions ourselves — the same approach Go's `go/ast.CommentMap` takes. This means we depend only on stable, already-shipped APIs in `onflow/cadence`.

The Swift-format influence applies to the **pipeline shape and rule architecture**, not to the printing algorithm. Swift-format uses Oppen 1979; we use Wadler 2003 because that's what `turbolent/prettier` implements and the Cadence AST already targets it.

---

## 3. Repository layout

```
cadencefmt/
├── cmd/
│   ├── cadencefmt/         # CLI entry point
│   │   └── main.go
│   └── cadencefmt-lsp/     # LSP server entry point
│       └── main.go
├── internal/
│   ├── format/             # Core formatter package
│   │   ├── formatter.go    # Public API: Format(src []byte, opts Options) ([]byte, error)
│   │   ├── pipeline.go     # Pipeline orchestration
│   │   ├── options.go      # Options struct + defaults
│   │   ├── trivia/         # Comment extraction & attachment
│   │   │   ├── scanner.go  # Re-scan source bytes for comments
│   │   │   ├── comment.go  # Comment type + classification
│   │   │   └── attach.go   # CommentMap construction & node→comments mapping
│   │   ├── rewrite/        # AST transformation passes
│   │   │   ├── rewrite.go  # Rewriter interface
│   │   │   ├── imports.go  # Sort & group imports
│   │   │   ├── strings.go  # Normalize string quotes
│   │   │   ├── modifiers.go # Canonicalize modifier ordering
│   │   │   └── parens.go   # Strip redundant parens
│   │   ├── render/         # AST → Doc → bytes
│   │   │   ├── render.go   # Top-level entry
│   │   │   ├── decl.go     # Declaration rendering (overrides ast.Doc where needed)
│   │   │   ├── stmt.go     # Statement rendering
│   │   │   ├── expr.go     # Expression rendering
│   │   │   ├── type.go     # Type rendering
│   │   │   └── trivia.go   # Comment interleaving in Doc tree
│   │   └── verify/         # Round-trip & idempotence checks (used in tests + --check)
│   │       └── verify.go
│   ├── config/             # Config file loading (.cadencefmt.toml)
│   │   ├── config.go
│   │   └── search.go       # Walk up dirs to find config
│   ├── lsp/                # LSP server implementation
│   │   ├── server.go
│   │   └── handlers.go
│   └── diff/               # Unified diff for --check / --diff
│       └── diff.go
├── testdata/
│   ├── format/             # snapshot tests: input.cdc + golden.cdc per case
│   ├── corpus/             # real-world contracts pulled from major Flow projects
│   └── fuzz/               # fuzz seeds
├── docs/
│   ├── style.md            # User-facing style guide
│   ├── config.md           # Config reference
│   └── editor-setup.md     # Neovim / Helix / VSCode (via generic LSP) setup
├── .github/workflows/
│   ├── ci.yml              # build + test on linux/mac/windows
│   └── release.yml         # goreleaser-driven releases
├── go.mod
├── go.sum
├── flake.nix
├── flake.lock
├── Makefile
├── README.md
└── LICENSE                 # Apache-2.0 to match upstream Cadence
```

Rationale for `internal/`: the public API surface is intentionally just the CLI and LSP. Library consumers should depend on `onflow/cadence` directly and use this repo's binaries, not vendor its packages.

---

## 4. Dependencies

### 4.1 Go module dependencies

Direct Go module dependencies (pin in `go.mod`):

- `github.com/onflow/cadence` — parser + AST. Track latest stable (currently `v1.10.x`).
- `github.com/turbolent/prettier` — Wadler-style pretty-printing IR; already used internally by `onflow/cadence`'s `Doc()` methods.
- `github.com/BurntSushi/toml` — config file parsing.
- `github.com/spf13/cobra` — CLI subcommands and flag handling. (Alternative: stdlib `flag` if we want zero deps; cobra preferred for the subcommand UX.)
- `go.lsp.dev/protocol` and `go.lsp.dev/jsonrpc2` — LSP types and transport. (Alternative: `github.com/sourcegraph/jsonrpc2` if simpler.)
- `github.com/google/go-cmp/cmp` — test assertions.
- `github.com/sergi/go-diff/diffmatchpatch` — unified diff for `--check`/`--diff` output.

Build-only:
- `github.com/goreleaser/goreleaser` — release artifacts.

No CGO. Target Go ≥ 1.22.

### 4.2 Development environment (Nix flake)

The repository ships a `flake.nix` that pins all non-Go tooling. This is the **canonical development environment** — contributors are expected to use it (or replicate the versions it pins). It is also the canonical Nix install path for end users (§13.3).

Entering the dev shell:

```bash
nix develop          # drops into a shell with Go, gopls, golangci-lint, goreleaser
nix flake check      # runs the project's checks (build + tests)
nix build            # builds the binaries reproducibly
nix run .# -- file.cdc       # runs cadencefmt on a file
nix run .#cadencefmt-lsp     # runs the LSP server
```

The flake provides four outputs per supported system (`x86_64-linux`, `aarch64-linux`, `x86_64-darwin`, `aarch64-darwin`):

| Output                          | Purpose                                                  |
|---------------------------------|----------------------------------------------------------|
| `packages.cadencefmt`           | Default; builds both binaries via `buildGoModule`        |
| `packages.cadencefmt-lsp`       | Alias to the same derivation, with `mainProgram = "cadencefmt-lsp"` for `nix run` |
| `apps.cadencefmt`, `apps.cadencefmt-lsp` | Wrappers so `nix run` picks the right binary    |
| `devShells.default`             | Dev shell described above                                |
| `checks.build`                  | Build verification (run by `nix flake check` and CI)     |
| `formatter`                     | `nixpkgs-fmt` for formatting `flake.nix` itself          |

Starter `flake.nix`:

```nix
{
  description = "cadencefmt — formatter for the Cadence smart contract language";

  inputs = {
    nixpkgs.url     = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs    = import nixpkgs { inherit system; };
        version = self.shortRev or "dev";
      in {
        packages = rec {
          cadencefmt = pkgs.buildGoModule {
            pname = "cadencefmt";
            inherit version;
            src = self;
            vendorHash = null; # populate after first `nix build`
            subPackages = [ "cmd/cadencefmt" "cmd/cadencefmt-lsp" ];
            ldflags = [ "-s" "-w" "-X main.version=${version}" ];
            meta = with pkgs.lib; {
              description = "Formatter for the Cadence smart contract language";
              license     = licenses.asl20;
              mainProgram = "cadencefmt";
              platforms   = platforms.unix;
            };
          };
          default = cadencefmt;
        };

        apps = {
          cadencefmt = flake-utils.lib.mkApp {
            drv  = self.packages.${system}.cadencefmt;
            name = "cadencefmt";
          };
          cadencefmt-lsp = flake-utils.lib.mkApp {
            drv  = self.packages.${system}.cadencefmt;
            name = "cadencefmt-lsp";
          };
          default = self.apps.${system}.cadencefmt;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_22 gopls gotools golangci-lint goreleaser
            git nixpkgs-fmt
          ];
        };

        checks.build = self.packages.${system}.cadencefmt;
        formatter    = pkgs.nixpkgs-fmt;
      });
}
```

Notes for the implementing agent:
- `vendorHash = null` is a placeholder. On first `nix build`, Nix prints the expected hash; copy it in. Update whenever `go.sum` changes.
- Keep `flake.lock` checked in. Update it intentionally with `nix flake update`, not as a side effect.
- The flake **does not** vendor or invoke `goreleaser`; goreleaser remains the release path for non-Nix users (§13.1) and is available in the dev shell only as a developer convenience.
- Non-Nix contributors are still first-class; the flake is one supported development path, not the only one. CI verifies both: a vanilla Go build matrix and a `nix flake check` job.

---

## 5. Architecture

### 5.1 Pipeline

```
source bytes
   │
   ├─► [1] parser.ParseProgram ──► *ast.Program
   │
   ├─► [2] trivia.Scan ──────────► []trivia.Comment
   │
   ├─► [3] trivia.Attach ────────► *CommentMap (node → leading/trailing/dangling comments)
   │
   ├─► [4] rewrite.Apply ────────► *ast.Program (mutated)
   │       (passes: imports, strings, modifiers, parens, …)
   │
   ├─► [5] render.Program ───────► prettier.Doc
   │       (calls ast.Element.Doc() augmented with comment interleaving)
   │
   ├─► [6] prettier.Prettier ────► formatted bytes
   │
   └─► [7] verify.RoundTrip ─────► (test mode only) confirm AST equivalence
```

Steps 1–3 produce an "annotated AST" (the AST plus a side-table of comments). Steps 4–6 are the formatting pipeline proper. Step 7 runs only in `--check` mode and in tests.

### 5.2 Module responsibilities

**`internal/format/trivia`** — extracts comments from source bytes and binds them to AST nodes by position. This is the most novel module; section 6 specifies it in detail.

**`internal/format/rewrite`** — implements `Rewriter` interface:

```go
type Rewriter interface {
    Name() string
    Rewrite(prog *ast.Program, cm *trivia.CommentMap) error
}
```

Each rewriter mutates the program (and updates the comment map if it moves nodes). Rewriters run sequentially in a fixed order. The fixed order matters for idempotence; document it in `rewrite.go` and never reorder without bumping a "format version" in config.

**`internal/format/render`** — turns an `*ast.Program` plus a `*CommentMap` into a `prettier.Doc`. Every AST element type has a corresponding `render*` function. Initial implementation delegates to the element's existing `Doc()` method and only overrides where we need different style (e.g. import grouping, trailing-comma rules, comment interleaving).

**`internal/format/verify`** — re-parses the formatted output and structurally compares the resulting AST to the original (modulo comment positions and whitespace-only token differences). Used as a final safety net before returning bytes to the caller. In production mode it can be opt-out (`--no-verify`) for speed; in CI and tests it always runs.

### 5.3 Public Go API (within the binary)

```go
package format

type Options struct {
    LineWidth      int       // default 100
    Indent         string    // default "    " (4 spaces)
    UseTabs        bool      // default false; if true, Indent is "\t"
    SortImports    bool      // default true
    QuoteStyle     QuoteStyle // default DoubleQuote
    StripSemicolons bool     // default true (where optional)
    KeepBlankLines int       // max consecutive blank lines preserved; default 1
    FormatVersion  string    // "1"; bumping is a breaking change
}

func Default() Options
func Format(src []byte, filename string, opts Options) ([]byte, error)
func Check(src []byte, filename string, opts Options) (formatted []byte, ok bool, err error)
```

`filename` is for diagnostics only; a file does not need to exist on disk.

---

## 6. Comment retention strategy

This is the technically novel part of the project.

### 6.1 Why out-of-band

The `onflow/cadence` parser does not retain comments in the AST. Issue #308 tracks adding trivia, but it isn't implemented. We refuse to fork the parser. Therefore we extract comments by re-scanning the source bytes ourselves and binding them to AST nodes by position — exactly the model `go/ast.CommentMap` uses.

### 6.2 Comment kinds

Cadence's lexical structure recognizes four comment kinds. Our scanner must produce all of them:

| Token            | Start      | End       | Notes                                    |
|------------------|------------|-----------|------------------------------------------|
| line             | `//`       | `\n` or EOF | Stops *before* the newline               |
| block            | `/*`       | `*/`      | **Nested** — Cadence supports `/* /* */ */` |
| doc-line         | `///`      | `\n` or EOF | Recognized as a separate kind for grouping |
| doc-block        | `/**`      | `*/`      | Nested. Distinguished from `/*` by exact start |

Disambiguation rules:
- `///` is doc-line only if the third character is *not* `/`. `////` is a normal line comment whose body begins with `/`.
- `/**` is doc-block only if the third character is *not* `*` and the comment is not the empty `/**/` (which is a normal empty block).

The scanner runs a small hand-written lexer (~150 lines) over the source bytes, emitting `Comment{Kind, Range, Text}` records. It must skip comment-like sequences inside string literals: `"// not a comment"`. The simplest correct approach is to track string literal state during the scan, including handling of escape sequences (`\"`) and string templates (`\(expr)`).

### 6.3 Comment record

```go
package trivia

type Kind int
const (
    KindLine Kind = iota
    KindBlock
    KindDocLine
    KindDocBlock
)

type Comment struct {
    Kind  Kind
    Range ast.Range // start + end position, byte offsets + line/col
    Text  string    // raw text including delimiters
}
```

Storing the raw text including delimiters is deliberate — when we later emit the comment we want byte-identical reproduction.

### 6.4 Grouping

Adjacent comments separated only by whitespace (not blank lines) form a `CommentGroup`. A blank line between two comments terminates the current group and starts a new one. This matters because a doc comment immediately above a declaration is one group; a copyright header at the top of the file separated by a blank line from an imports block is a different group.

```go
type CommentGroup struct {
    Comments []Comment
    Range    ast.Range
}
```

### 6.5 Position-based attachment

After grouping, we walk the AST and attach groups to nodes. The algorithm is a single pass that merges two sorted streams: (a) comment groups in source order, and (b) AST nodes in pre-order traversal.

For each comment group `G` with range `[gs, ge]`:

1. Find the smallest AST node `N` whose range fully contains `G`. Attach to `N`.
2. Within that `N`, classify the group:
   - If `ge` precedes the start of any child of `N` → `Leading` of `N` (or of the first child, depending on heuristic; see 6.6).
   - If `gs` follows the end of all children of `N` → `Trailing` of `N` (or of the last child).
   - If between two children → `Trailing` of the preceding child OR `Leading` of the following child (see 6.6).
   - If on the same line as the end of a child and no other content separates them → `SameLine` of that child (this is the end-of-line comment case).

3. Comments before the first declaration in `*ast.Program` and after the last declaration are file-level "header" and "footer" comment groups, stored on `CommentMap` directly.

### 6.6 Disambiguation heuristic

The "between two children" case is the hairy one. Use this heuristic, in order:

1. **Same-line wins.** If the comment's start line equals the previous child's end line and the comment's end line equals the next child's start line minus 1 → it's a trailing same-line comment on the previous child.
2. **Blank line separates.** If there's a blank line between the previous child and the comment → the comment is `Leading` of the next child.
3. **Otherwise** → `Trailing` of the previous child.

This mirrors gofmt and matches reader intuition: a comment hugging the line above belongs to that line; a comment with breathing room belongs to what follows.

### 6.7 The CommentMap structure

```go
type CommentMap struct {
    Header []*CommentGroup // file top, before first decl
    Footer []*CommentGroup // file bottom, after last decl

    Leading  map[ast.Element][]*CommentGroup
    Trailing map[ast.Element][]*CommentGroup
    SameLine map[ast.Element]*CommentGroup // at most one per node
}

func (cm *CommentMap) Take(n ast.Element) (leading, sameLine, trailing []*CommentGroup)
```

`Take` returns and removes the comments associated with a node — this is how the renderer ensures every comment is emitted exactly once. After rendering completes, the formatter asserts the map is empty; any leftover comments indicate a bug and cause the formatter to error out (`internal error: orphaned comments at <pos>`) rather than silently drop them.

### 6.8 Interleaving comments into the Doc tree

In `render/trivia.go`:

```go
func wrapWithComments(elem ast.Element, doc prettier.Doc, cm *trivia.CommentMap) prettier.Doc {
    leading, sameLine, trailing := cm.Take(elem)
    parts := prettier.Concat{}
    for _, g := range leading {
        parts = append(parts, renderCommentGroup(g), prettier.HardLine{})
    }
    parts = append(parts, doc)
    if sameLine != nil {
        parts = append(parts, prettier.Text("  "), renderCommentGroupInline(sameLine))
    }
    for _, g := range trailing {
        parts = append(parts, prettier.HardLine{}, renderCommentGroup(g))
    }
    return parts
}
```

`HardLine` is used (not `Line`) because comments must not be elided when a group flattens. Doc-block comments (`/** … */`) need special handling for re-flow — see §8.7.

---

## 7. Pretty-printing IR

We use `turbolent/prettier`'s document algebra unchanged:

| Constructor       | Behavior                                                                        |
|-------------------|---------------------------------------------------------------------------------|
| `Text(s)`         | Literal string, must contain no newlines                                        |
| `Space`           | A single space (= `Text(" ")`)                                                  |
| `Line{}`          | Line break; flattens to a space                                                 |
| `SoftLine{}`      | Line break; flattens to nothing                                                 |
| `HardLine{}`      | Line break; never flattens (always breaks)                                      |
| `Indent{Doc}`     | Increase indentation level for nested doc                                       |
| `Dedent{Doc}`     | Decrease indentation level for nested doc                                       |
| `Concat{...}`     | Sequence of docs                                                                |
| `Group{Doc}`      | Try to flatten; if it fits in the remaining width, do so; else lay out broken   |

Render entry point:

```go
prettier.Prettier(writer, doc, opts.LineWidth, opts.Indent)
```

We do **not** add new IR primitives. If we need behavior beyond this (e.g. fill-mode wrapping for long argument lists), we encode it with `Group` + nested `Group` patterns following the recipes in Wadler's paper.

---

## 8. Style rules

Default style. All values configurable via `cadencefmt.toml`.

### 8.1 Line width
Default 100. Soft constraint: `Group` collapses to one line if it fits, otherwise breaks at every `Line` it contains.

### 8.2 Indentation
4 spaces. Tabs supported via `use_tabs = true`. Mixed indentation is never emitted.

### 8.3 Imports

Imports are normalized into groups separated by a single blank line, in this order:

1. **Standard imports**: identifier-only imports of contracts at well-known addresses (e.g. `import FungibleToken from 0x…`).
2. **Address imports** by address, sorted lexicographically by address then identifier.
3. **String imports** (relative paths), sorted lexicographically.

Within each group, imports are sorted lexicographically by the imported identifier. Duplicate imports are collapsed.

```cadence
import Crypto

import FungibleToken from 0x9a0766d93b6608b7
import NonFungibleToken from 0x631e88ae7f1d7c20

import "MyContract"
```

### 8.4 Access modifiers and ordering

For declarations with multiple modifiers (e.g. `access(all) view fun`), enforce a canonical order:

```
access-modifier  →  static  →  native  →  view  →  kind-keyword
```

E.g. `view access(all) fun foo` becomes `access(all) view fun foo`.

The deprecated `pub`, `pub(set)`, `priv` modifiers are **not** rewritten by the formatter — running formatter and migration tools should be orthogonal. We render them as written. (A separate `--migrate` mode could be added in v2 but is out of scope.)

### 8.5 Resource operators

`<-`, `<-!`, `<- create`, `destroy` are surrounded by a single space on each side (where syntactically valid). Examples:

```cadence
let vault <- create Vault(balance: 100.0)
self.vaults[id] <-! vault
let old <- self.vaults.remove(key: id)
destroy old
```

`<-` after `=` does not get a leading space (the assignment operator already provides one): write `let v <- create X()` not `let v <-create X()` and not `let v < - create X()`.

### 8.6 Function declarations and calls

A function declaration's signature is one `Group`. If it fits on one line, emit on one line; otherwise break after `(` and put each parameter on its own line, with a trailing comma:

```cadence
// fits
access(all) fun transfer(amount: UFix64, to: Address) { … }

// doesn't fit
access(all) fun transferWithMemoAndAuthorization(
    amount: UFix64,
    to: Address,
    memo: String,
    auth: auth(Withdraw) &Vault,
) { … }
```

Trailing comma in multi-line parameter and argument lists is **always emitted**. Single-line never has trailing comma.

The same rule applies to call expressions, array/dictionary literals, and generic type argument lists.

### 8.7 Pre/post conditions

`pre` and `post` blocks render with each condition on its own line, indented one level beyond the keyword:

```cadence
access(all) fun withdraw(amount: UFix64): @Vault {
    pre {
        amount > 0.0: "amount must be positive"
        amount <= self.balance: "insufficient funds"
    }
    post {
        result.balance == amount: "incorrect withdrawal"
    }
    // body
}
```

### 8.8 Composite type bodies

Composite declarations (`contract`, `resource`, `struct`, `attachment`, `enum`, `event`, `interface`) place the opening brace on the same line as the declaration head. Members are separated by blank lines if any member spans multiple lines, otherwise compact.

```cadence
access(all) resource Vault: Provider, Receiver, Balance {

    access(all) var balance: UFix64

    init(balance: UFix64) {
        self.balance = balance
    }

    access(Withdraw) fun withdraw(amount: UFix64): @Vault {
        self.balance = self.balance - amount
        return <- create Vault(balance: amount)
    }
}
```

Single-line bodies — `event Foo()`, empty interfaces — render compactly: `access(all) event Withdraw(amount: UFix64, from: Address?)`.

### 8.9 Casting

`as`, `as?`, `as!` get a single space on each side. Long casts can break before the `as`:

```cadence
let typed = self.account.capabilities.borrow<&Vault>(/public/MainVault)
    as &{Provider, Balance}
```

### 8.10 String literals

Single-quoted strings are not valid Cadence; the formatter does not need to convert them. String escapes are emitted as written (no normalization of `\u{…}` vs literal characters — that's a semantic change).

String templates `"hello \(name)"` are emitted as written. The interpolated expression inside `\( … )` is **not** re-formatted in v1; the bytes between `\(` and the matching `)` are preserved verbatim. (Re-formatting interpolations is a v2 feature.)

### 8.11 Comments

- Line comments `//` are emitted verbatim, with the comment text trimmed of trailing whitespace.
- Block comments `/* */` are emitted verbatim with no internal reflow.
- Doc-line comments `///` are emitted verbatim.
- Doc-block comments `/**` are emitted verbatim with one normalization: if the comment uses the "leading-asterisk" style (`* foo` on each interior line), the leading whitespace of those lines is normalized to align the `*` characters with the column of the opening `/**`'s `*`.

End-of-line comments (`SameLine` in the comment map) are separated from the preceding code by exactly two spaces:

```cadence
let x = 1  // trailing comment
```

Blank lines: at most `KeepBlankLines` (default 1) consecutive blank lines are preserved between top-level declarations. Inside function bodies, blank lines are preserved up to the same limit.

### 8.12 Pragmas

`#allowAccountLinking`, `#removedType`, etc. render flush-left at the top of the file, in the order they appeared, after any header comment block and before imports.

### 8.13 Transactions

```cadence
transaction(amount: UFix64, recipient: Address) {

    let sentVault: @{FungibleToken.Vault}

    prepare(signer: auth(BorrowValue) &Account) {
        let vaultRef = signer.storage.borrow<auth(Withdraw) &ExampleToken.Vault>(
            from: /storage/exampleTokenVault,
        ) ?? panic("Could not borrow reference to the owner's Vault!")

        self.sentVault <- vaultRef.withdraw(amount: amount)
    }

    execute {
        let recipient = getAccount(recipient)
        let receiverRef = recipient.capabilities.borrow<&{FungibleToken.Receiver}>(
            /public/exampleTokenReceiver,
        ) ?? panic("Could not borrow receiver reference")

        receiverRef.deposit(from: <-self.sentVault)
    }
}
```

`prepare`, `pre`, `execute`, `post` blocks are separated by a blank line if any of them spans multiple lines.

---

## 9. Configuration

### 9.1 File format

`cadencefmt.toml` (TOML chosen for the Go ecosystem and human friendliness). Discoverable: walk up from the input file's directory until found, or use `$XDG_CONFIG_HOME/cadencefmt/config.toml`, or fall back to defaults.

```toml
# cadencefmt.toml — all keys optional; defaults shown.

format_version = "1"

[format]
line_width        = 100
indent            = "    "      # four spaces
use_tabs          = false
sort_imports      = true
quote_style       = "double"    # only "double" supported in v1
strip_semicolons  = true
keep_blank_lines  = 1

[format.imports]
group_standard_first = true
```

Unknown keys produce a warning, not an error, so future-version configs don't break older binaries on minor releases. Unknown values for known keys produce an error.

### 9.2 Format version

Bumping `format_version` (or the absence implying `"1"`) is the mechanism for breaking style changes. v1 binaries refuse to read configs with `format_version` > `"1"` and exit 2 with a clear message.

### 9.3 Per-file overrides

Out of scope for v1. Add later as `[[overrides]]` sections matching glob patterns.

---

## 10. CLI design

Binary name: `cadencefmt`.

### 10.1 Usage

```
cadencefmt [flags] [path…]
```

Behavior:
- No paths and stdin is a TTY: print help and exit 2.
- No paths and stdin is a pipe: read from stdin, write formatted output to stdout.
- One or more paths: format each `.cdc` file in place. Directories are walked recursively for `.cdc` files. Symlinks are not followed.

### 10.2 Flags

| Flag                | Default | Description                                                        |
|---------------------|---------|--------------------------------------------------------------------|
| `-w`, `--write`     | false   | Write changes back to source files. Without this, output goes to stdout (single file) or no-op (multi-file). |
| `-c`, `--check`     | false   | Exit 1 if any input would change. Print the offending paths.       |
| `-d`, `--diff`      | false   | Print unified diff of changes instead of formatted output.         |
| `--config FILE`     | auto    | Use this config file. Disables auto-discovery.                     |
| `--no-config`       | false   | Ignore any config file; use defaults plus other flags.             |
| `--stdin-filename`  | `""`    | Filename to use for diagnostics when reading stdin.                |
| `--no-verify`       | false   | Skip the round-trip AST equivalence check. Faster, less safe.      |
| `--version`         |         | Print version and exit.                                            |
| `--help`, `-h`      |         | Print help and exit.                                               |

### 10.3 Exit codes

| Code | Meaning                                                |
|------|--------------------------------------------------------|
| 0    | Success; no changes needed (or changes written if `-w`) |
| 1    | `--check` mode: at least one file would change         |
| 2    | Usage error (bad flags, missing input, etc.)           |
| 3    | Parse error in input                                   |
| 4    | Internal error (verify failed, orphaned comments, etc.) |

### 10.4 Subcommands

None in v1. The CLI is single-action. (Subcommands like `cadencefmt lsp` were considered; instead the LSP server is a separate binary `cadencefmt-lsp` so users who don't need it don't ship it.)

### 10.5 Examples

```bash
# Format a file in place
cadencefmt -w contract.cdc

# Format all .cdc files in cwd, recursively, in place
cadencefmt -w .

# Check formatting in CI; exit 1 if anything would change
cadencefmt -c .

# Editor integration: pipe through stdin
cat contract.cdc | cadencefmt --stdin-filename contract.cdc
```

---

## 11. LSP server

Binary name: `cadencefmt-lsp`. Speaks LSP over stdio.

### 11.1 Capabilities

Server announces:

```json
{
  "capabilities": {
    "documentFormattingProvider": true,
    "textDocumentSync": { "openClose": true, "change": 1 }
  }
}
```

(`change: 1` is full-document sync — sufficient for a formatting-only server.)

### 11.2 Request handling

- `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didClose` — maintain in-memory document state.
- `textDocument/formatting` — run `format.Format(document, options)`. Return a single `TextEdit` covering the whole document with the new text.
- All other requests respond with `MethodNotFound`.

### 11.3 Configuration discovery

On `initialize`, walk up from `rootUri` to find `cadencefmt.toml`. Re-read the config on `workspace/didChangeConfiguration` and on `workspace/didChangeWatchedFiles` for the config path.

### 11.4 v1.1 additions (out of v1 scope)
- `textDocument/rangeFormatting` — once implemented, requires the formatter to support partial input. The simplest correct implementation is: format the whole document, compute the smallest enclosing top-level declarations covering the range, and return only edits within that span.
- `textDocument/onTypeFormatting` — likely never; would require incremental parsing the official parser doesn't expose.

---

## 12. Editor integrations

This project ships **no editor extensions in v1.** All integration is via documented setup using each editor's existing formatter or LSP support. Maintaining a bespoke VSCode extension is explicitly deferred (§1.2).

### 12.1 Neovim

No plugin required. Documented setup using `conform.nvim`:

```lua
require("conform").setup({
  formatters_by_ft = {
    cadence = { "cadencefmt" },
  },
  formatters = {
    cadencefmt = {
      command = "cadencefmt",
      args = { "--stdin-filename", "$FILENAME" },
      stdin = true,
    },
  },
})
```

For LSP-based formatting, document the standard `vim.lsp.start` snippet pointing at `cadencefmt-lsp`.

### 12.2 Helix

Documented in `docs/editor-setup.md` with a `languages.toml` snippet:

```toml
[[language]]
name = "cadence"
formatter = { command = "cadencefmt", args = ["--stdin-filename", "buffer.cdc"] }
auto-format = true
```

### 12.3 VSCode

No extension is shipped by this project. Two supported paths, both documented in `docs/editor-setup.md`:

1. **Generic LSP client.** Install an extension that registers `cadencefmt-lsp` against the `cadence` language id (e.g. `llllvvuu.llllvvuu-glspc` or similar generic LSP-bridge extensions). This is the preferred path; it gives format-on-save behavior identical to Neovim/Helix users.
2. **Format-on-save shell-out.** Use a generic "run command on save" extension (e.g. `emeraldwalk.runonsave`) configured to pipe the buffer through `cadencefmt --stdin-filename ${file}`.

If the existing `onflow.cadence` extension is installed, both options coexist with it — that extension does not currently provide formatting, so there is no conflict.

### 12.4 Other

Any editor that can shell out to a stdin/stdout command works without further integration. We do not maintain integrations beyond the three above.

---

## 13. Distribution & installation

The formatter is distributed through three channels. The Nix flake is the primary "blessed" path because the dev environment and the install path share the same derivation, which means the binary a user installs is the same one the project's CI verifies.

### 13.1 GitHub Releases (binary downloads)

Driven by `goreleaser` (§15, Phase 12). Each tag publishes static binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`, plus checksums and SBOMs. Both `cadencefmt` and `cadencefmt-lsp` are included in each archive.

### 13.2 `go install`

```bash
go install github.com/<owner>/cadencefmt/cmd/cadencefmt@latest
go install github.com/<owner>/cadencefmt/cmd/cadencefmt-lsp@latest
```

This is supported but discouraged for non-developer users — it requires a Go toolchain and produces a binary that isn't checksum-pinned. Document it in the README under "for Go developers."

### 13.3 Nix flake

The same `flake.nix` used for development (§4.2) is also the install path for Nix users. There is no separate package definition — installing locks you to the same derivation contributors test against.

**Try it without installing:**
```bash
nix run github:<owner>/cadencefmt -- file.cdc
nix run github:<owner>/cadencefmt#cadencefmt-lsp
```

**Install into your user profile:**
```bash
nix profile install github:<owner>/cadencefmt
```

**NixOS system configuration (flake-based):**
```nix
# flake.nix
{
  inputs.cadencefmt.url = "github:<owner>/cadencefmt";

  outputs = { self, nixpkgs, cadencefmt, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ({ pkgs, ... }: {
          environment.systemPackages = [
            cadencefmt.packages.${pkgs.system}.default
          ];
        })
      ];
    };
  };
}
```

**Home Manager:**
```nix
{
  inputs.cadencefmt.url = "github:<owner>/cadencefmt";

  outputs = { self, home-manager, cadencefmt, ... }: {
    homeConfigurations."user" = home-manager.lib.homeManagerConfiguration {
      modules = [
        ({ pkgs, ... }: {
          home.packages = [
            cadencefmt.packages.${pkgs.system}.default
          ];
        })
      ];
    };
  };
}
```

**Pin to a specific tag** for reproducible installs:
```nix
inputs.cadencefmt.url = "github:<owner>/cadencefmt/v1.0.0";
```

### 13.4 nixpkgs

Out of scope for v1. Once v1 is released and stable for ~3 months, submitting the package to `nixpkgs` is reasonable. The flake derivation is structured to translate cleanly to a `pkgs/by-name/ca/cadencefmt/package.nix` entry — `buildGoModule` is the same in both places. Tracking issue: open one in our own repo titled "Submit to nixpkgs" so it's not lost.

### 13.5 Homebrew

Out of scope for v1. A `homebrew-tap` repository can be added post-1.0 if there is demand from macOS users who don't use Nix.

---

## 14. Testing strategy

### 14.1 Unit tests

Per-module Go tests using stdlib `testing`. Coverage target: 85% line coverage for `internal/format/*`. Comment attachment and rewrite passes get exhaustive table tests.

### 14.2 Snapshot tests

Under `testdata/format/`, each test case is a directory containing `input.cdc` and `golden.cdc`. The test harness formats `input.cdc`, compares to `golden.cdc`, and uses `-update` flag to refresh goldens. Example structure:

```
testdata/format/
  hello-world/
    input.cdc
    golden.cdc
  resource-with-pre-conditions/
    input.cdc
    golden.cdc
  long-function-signature/
    input.cdc
    golden.cdc
  comments-leading-trailing-sameline/
    input.cdc
    golden.cdc
  …
```

Minimum case coverage in v1: every declaration kind, every statement kind, every expression kind, every type kind, and every comment kind in every position class. Estimated count: ~150 cases.

### 14.3 Idempotence tests

For every snapshot test, also assert `format(format(input)) == format(input)`. Run as a separate test pass in CI.

### 14.4 Round-trip AST equivalence

For every snapshot test, parse both `input` and `formatted`, walk both ASTs in parallel, and assert that every node has the same kind and the same significant fields (excluding position info). Comments are excluded from this check (their preservation is checked separately).

### 14.5 Real-world corpus

Pull the following repositories into `testdata/corpus/` (read-only fixtures, vendored as git submodules or download script):

- `onflow/flow-core-contracts` — staking, identity, fungible token core contracts
- `onflow/flow-ft` — Fungible Token standard
- `onflow/flow-nft` — Non-Fungible Token standard
- `flow-foundation/cadence-libraries` — utility libraries
- A handful of major NFT projects' public contracts (NBA Top Shot, NFL All Day) where licensing permits

For each `.cdc` file in the corpus:
1. Format succeeds (no parse errors, no internal errors).
2. Formatter is idempotent.
3. AST round-trip holds.

We do not assert against goldens for corpus files — those are tracked by snapshot tests only. The corpus verifies "doesn't crash on real code."

### 14.6 Comment preservation tests

A dedicated test suite that, for each snapshot case, extracts all comments from input and from output and asserts the multisets are equal (modulo the doc-block leading-asterisk normalization in §8.11).

### 14.7 Fuzzing

Use Go's native `testing.F` fuzzer. Two fuzz targets:

- `FuzzFormat`: feed arbitrary bytes, assert the formatter never panics. Parse failures are expected and ignored.
- `FuzzRoundtrip`: feed bytes that parse successfully (gated on parser success), assert idempotence and AST round-trip.

Seed corpus: every file from `testdata/format/` and `testdata/corpus/`. Run fuzzing in CI on a 5-minute budget per target per push.

### 14.8 CI matrix

GitHub Actions, two parallel pipelines:

- **Vanilla Go matrix:** `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`. Go `1.22` and `1.23`. Builds, runs unit tests, snapshot tests, and idempotence tests.
- **Nix flake job:** runs `nix flake check` on `linux/amd64` and `darwin/amd64`. This validates that `flake.nix` builds, that `vendorHash` is current, and that the dev shell resolves. A failure here means contributors using `nix develop` would be broken even if the vanilla Go build is fine.

All jobs across both pipelines must pass before merge.

---

## 15. Implementation phases

Each phase produces a working, testable deliverable. Total effort estimate: 4–6 weeks for a single experienced contributor.

### Phase 1 — Skeleton (2 days)
- Repo layout per §3
- `go.mod` with dependencies
- `flake.nix` with the starter content from §4.2; verify `nix develop` enters a working shell and `nix build` produces a binary (set `vendorHash` after the first build error reports it)
- `cmd/cadencefmt/main.go` reads stdin, parses with `parser.ParseProgram`, prints AST kind, exits 0
- CI pipeline green for both the vanilla Go matrix and the `nix flake check` job (§14.8)
- Apache-2.0 LICENSE, README with project status "alpha" and quick-start instructions for both `go install` and `nix run`

### Phase 2 — Naive renderer (3 days)
- Wire `prog.Doc()` directly to `prettier.Prettier` for stdout
- No comment handling; no rewrites
- Snapshot test harness in place with 5 hand-written cases
- This phase confirms end-to-end plumbing works on AST elements that already implement `Doc()`

### Phase 3 — Comment scanner (3 days)
- Implement `internal/format/trivia/scanner.go`: hand-written lexer for the four comment kinds
- Comprehensive table tests including string-template edge cases
- No attachment yet; just produce `[]Comment` from source bytes

### Phase 4 — Comment attachment (4 days)
- Implement `CommentMap` and the merge-walk algorithm in §6.5
- Implement disambiguation heuristic §6.6
- Test against hand-written cases covering every position class
- Still no rendering of comments

### Phase 5 — Comment interleaving (3 days)
- `render/trivia.go`: wrap each AST element's Doc with leading/trailing/same-line comments from the CommentMap
- The renderer must call `CommentMap.Take` exactly once per element
- Final assertion: empty CommentMap after rendering, else error
- Snapshot tests for §8.11 cases

### Phase 6 — Rewrite passes (4 days)
- `rewrite/imports.go`: sort and group imports
- `rewrite/strings.go`: nothing in v1 (placeholder; reserved for v2 interpolation reformatting)
- `rewrite/modifiers.go`: canonicalize modifier ordering
- `rewrite/parens.go`: strip redundant parens (carefully — only where the parse is unambiguous without them)
- Each rewriter has its own snapshot test cases

### Phase 7 — Style overrides in renderer (5 days)
- For every AST element where the upstream `Doc()` doesn't match our style, write a custom `render*` function in `render/{decl,stmt,expr,type}.go`
- This is the bulk of the line-count work; ~50 functions, mostly small
- Snapshot test coverage extends to every declaration/statement/expression kind

### Phase 8 — CLI polish (2 days)
- All flags from §10 implemented
- `--check` and `--diff` modes
- Recursive directory walking
- Exit codes per §10.3

### Phase 9 — LSP server (3 days)
- `cmd/cadencefmt-lsp/main.go`
- `textDocument/formatting` end-to-end
- Tested manually against Neovim and Helix before tagging v1

### Phase 10 — Verify pass & idempotence guarantees (2 days)
- `internal/format/verify`
- AST equivalence walker
- `--no-verify` flag
- Idempotence test pass added to CI

### Phase 11 — Corpus testing and bug fixing (1 week)
- Run on the corpus (§14.5)
- File and fix bugs discovered
- Track corpus pass rate over time

### Phase 12 — Release (2 days)
- `goreleaser` config; binaries for the CI matrix
- GitHub Releases as the primary binary distribution channel (§13.1)
- Verify the Nix install path against the tagged commit: `nix run github:<owner>/cadencefmt/v1.0.0 -- testdata/format/hello-world/input.cdc` produces the expected output (§13.3)
- Homebrew tap and `nixpkgs` submission deferred (§13.4, §13.5)
- Tag v1.0.0
- Announcement comment on `onflow/cadence#209` linking to the release

---

## 16. Resolved decisions and remaining open questions

### Resolved
- **Module path / hosting:** the project lives in a personal GitHub repo for v1 (path: `github.com/<owner>/cadencefmt`, owner TBD by the implementer). Transferring to `onflow/` is deferred and will be reconsidered only after the formatter is stable, has corpus-validated correctness, and is wanted by the upstream team. **No upstream-permission gate is required to start.**
- **License:** Apache-2.0, matching upstream Cadence.
- **`pub`/`priv` migration:** out of scope; emit as written.
- **Editor extensions:** none in v1. Editor support is documented setup against the CLI and LSP server (§12).

### Still open (decide before merging Phase 1)
1. **Multiline string handling:** Cadence allows multiline strings via concatenation or interpolation, but no triple-quoted form. Confirm with a corpus scan that there are no edge cases the lexer needs to know about that aren't already handled by the parser.

2. **Format-off directives:** support `// cadencefmt:off` / `// cadencefmt:on` ranges? **Recommendation:** yes, in v1.1, not v1. v1 always formats.

3. **Binary name on collision:** if `cadencefmt` clashes with anything in the Flow ecosystem already, fall back to `cdcfmt`. Quick search before Phase 1; trivially resolved either way.

---

## 17. Upstream coordination

Optional but recommended once Phase 6 or 7 is reached and the formatter produces visibly useful output: comment on [`onflow/cadence#209`](https://github.com/onflow/cadence/issues/209) with:

- Link to the personal repo
- Brief design summary (essentially §2 of this spec)
- Stating intent: build out-of-tree on a personal GitHub; if the Cadence team wants to adopt it later, the code is Apache-2.0 and structured to be vendored.
- Asking specifically: any objection to the out-of-band comment scanner approach vs. waiting for #308?

This is for visibility and to surface any partially completed work in `bastian/*` branches that might be relevant. It is not a blocker for any phase. Phase 1 starts immediately on the personal repo without waiting for a response.

---

## Appendix A: Cadence language surface to handle

This list drives test case coverage. Every entry must have at least one snapshot test exercising it.

**Top-level declarations**
- `import` (identifier-only, address, string)
- pragma directives (`#allowAccountLinking`, `#removedType`, …)
- `access(all) contract`, `access(all) contract interface`
- `access(all) resource`, `access(all) resource interface`
- `access(all) struct`, `access(all) struct interface`
- `access(all) attachment`
- `access(all) event`
- `access(all) enum`
- `access(all) entitlement`, `access(all) entitlement mapping`
- `access(all) fun`, `view fun`, `native fun`
- `access(all) let`, `access(all) var`
- `transaction { … }`

**Modifiers and access**
- `access(all)`, `access(self)`, `access(contract)`, `access(account)`
- `access(E1, E2)` entitlement set, `access(E1 | E2)` disjunction
- `access(mapping M)` entitlement mapping
- `view`, `native`, `static`
- Deprecated: `pub`, `pub(set)`, `priv` — preserved as written

**Types**
- Primitives: `Bool`, `Int*`, `UInt*`, `Word*`, `Fix64`, `UFix64`, `Address`, `String`, `Character`, `Path`, `AnyStruct`, `AnyResource`, `Never`, `Void`
- `T?` (optional), `&T`, `auth(E) &T`, `&T?`
- `[T]`, `[T; N]`, `{K: V}`
- Function types `fun(T): U`, `view fun(T): U`
- Restricted/intersection types `{I1, I2}`, `T{I1, I2}` (deprecated)
- `Capability`, `Capability<&T>`
- Reference types with intersection `auth(E) &{I1, I2}`

**Statements**
- Variable declarations `let`, `var` with optional type annotation
- Assignment `=`, compound assignments `+=`, `-=`, `*=`, `/=`, `%=`
- Move assignment `<-`, force-move `<-!`, swap `<->`
- `if`, `if let`, `if var`, `else if`, `else`
- `switch` with cases (no fallthrough)
- `while`, `for in`
- `return`, `break`, `continue`
- `emit`
- `destroy`
- Block `{ … }`

**Expressions**
- Literals: int, fix, string (with templates), bool, nil, address, path
- Identifier reference
- Member access `.`, optional chaining `?.`
- Index `[…]`
- Function call `f(args)`, with named args
- `create T(…)`, `<- create T(…)`
- Type expressions `Type<T>()`
- Conditional `cond ? a : b`
- Binary ops, unary ops, force-unwrap `!`
- Casting `as`, `as?`, `as!`
- Reference `&v as &T`
- Array literal `[…]`, dictionary literal `{…: …}`
- Path expressions `/storage/foo`, `/public/foo`, `/private/foo`
- Pre/post conditions inside function bodies

**Comments**
- `//`, `/* */`, `///`, `/**`, nested block comments
- Same-line trailing
- Leading on declarations
- Doc comments preceding declarations
- File header
- File footer
- Inside parameter lists
- Between case clauses
- Inside expressions (rare but legal in some positions)

---

## Appendix B: Module file inventory

Files an implementing agent should create, with one-line responsibility statements.

```
cmd/cadencefmt/main.go            CLI entry; flag parsing; dispatch to format.Format
cmd/cadencefmt-lsp/main.go        LSP entry; jsonrpc loop

internal/format/formatter.go      Public Format/Check; orchestrates pipeline
internal/format/pipeline.go       Pipeline runner; each phase named & timed
internal/format/options.go        Options struct; Default(); validation

internal/format/trivia/scanner.go Comment lexer over source bytes
internal/format/trivia/comment.go Comment, CommentGroup types; classification helpers
internal/format/trivia/attach.go  Build CommentMap by walking AST + comments

internal/format/rewrite/rewrite.go    Rewriter interface; runner
internal/format/rewrite/imports.go    Import sort + group
internal/format/rewrite/modifiers.go  Canonical modifier ordering
internal/format/rewrite/parens.go     Strip redundant parens

internal/format/render/render.go      Top-level Program rendering
internal/format/render/decl.go        Declaration rendering overrides
internal/format/render/stmt.go        Statement rendering overrides
internal/format/render/expr.go        Expression rendering overrides
internal/format/render/type.go        Type rendering overrides
internal/format/render/trivia.go      Comment interleaving into Doc tree

internal/format/verify/verify.go      AST equivalence walker; orphan check

internal/config/config.go             Config struct; Load
internal/config/search.go             Walk-up search; XDG fallback

internal/lsp/server.go                LSP handlers map
internal/lsp/handlers.go              Per-method handlers

internal/diff/diff.go                 Unified diff for --diff/--check output
```

End of specification.
