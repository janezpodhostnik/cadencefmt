# cadencefmt — Implementation Prompt for AI Agent

You are building `cadencefmt`, a deterministic formatter for the Cadence smart contract language (Flow blockchain). The repository is at `github.com/janezpodhostnik/cadencefmt`. The full technical specification is in `docs/cadencefmt-SPEC.md` — that document is the source of truth for all design decisions, style rules, and behavioral requirements. Read it before starting any phase.

This prompt tells you **how to build it** — what order, what to verify, what to avoid, and how to hand off between sessions.

---

## Resuming from a previous session

At the start of every session:

1. Read `PROGRESS.md` at repo root. It tracks which phases are complete.
2. Run `git log --oneline -20` to see recent commits.
3. Run `go build ./...` and `go test ./...` to check current state.
4. Pick up at the first incomplete phase.

If `PROGRESS.md` doesn't exist, start at Phase 1.

After completing each phase, update `PROGRESS.md` with a checked box, commit it, and push. This is how the next session knows where you are.

---

## Hard invariants

These must hold at all times once the formatter produces output (Phase 2+):

1. **Round-trip correctness**: `parse(format(S))` produces a structurally equivalent AST to `parse(S)` for any input that parses without error.
2. **Idempotence**: `format(format(S)) == format(S)`, byte-for-byte.
3. **Comment preservation**: every comment in input appears exactly once in output, at the same logical position.
4. **Fail-safe**: parse errors cause a non-zero exit, error on stderr, nothing on stdout.

Implement tests for these from Phase 2 onward. They run on every snapshot test case automatically.

---

## Do NOT

- Fork or modify the `onflow/cadence` parser. Use it as a library only.
- Add new IR primitives to `turbolent/prettier`. Use the existing algebra (`Text`, `Line`, `HardLine`, `SoftLine`, `Group`, `Indent`, `Dedent`, `Concat`, `Space`).
- Rewrite deprecated `pub`/`pub(set)`/`priv` modifiers. Emit them as written.
- Reformat expressions inside string template interpolations `\(...)`. Preserve verbatim.
- Export any packages under `internal/`. The public surface is CLI + LSP binaries only.
- Reorder rewrite passes without bumping `format_version` in config.
- Silently drop comments. If the CommentMap is non-empty after rendering, that's an internal error (exit code 4), not something to sweep under the rug.

---

## Go module path and dependencies

```
module github.com/janezpodhostnik/cadencefmt
go 1.22
```

Direct dependencies:
- `github.com/onflow/cadence` — parser + AST (track latest stable, currently v1.10.x)
- `github.com/turbolent/prettier` — pretty-printing IR (already a transitive dep of cadence)
- `github.com/BurntSushi/toml` — config parsing
- `github.com/spf13/cobra` — CLI flags
- `github.com/google/go-cmp/cmp` — test assertions
- `github.com/sergi/go-diff/diffmatchpatch` — unified diff output

No CGO. No LSP dependencies until Phase 9.

---

## Repository layout

Follow this structure exactly. See spec §3 and Appendix B for file responsibilities.

```
cmd/cadencefmt/main.go
cmd/cadencefmt-lsp/main.go
internal/format/formatter.go      # Format(), Check()
internal/format/pipeline.go       # pipeline orchestration
internal/format/options.go        # Options struct + Default()
internal/format/trivia/scanner.go # comment lexer
internal/format/trivia/comment.go # Comment, CommentGroup, Kind types
internal/format/trivia/attach.go  # CommentMap construction
internal/format/rewrite/rewrite.go
internal/format/rewrite/imports.go
internal/format/rewrite/modifiers.go
internal/format/rewrite/parens.go
internal/format/render/render.go
internal/format/render/decl.go
internal/format/render/stmt.go
internal/format/render/expr.go
internal/format/render/type.go
internal/format/render/trivia.go  # comment interleaving
internal/format/verify/verify.go
internal/config/config.go
internal/config/search.go
internal/lsp/server.go
internal/lsp/handlers.go
internal/diff/diff.go
testdata/format/*/input.cdc + golden.cdc
```

---

## Pipeline overview

This is the core data flow. Understand it before writing any code.

```
source []byte
  │
  ├─► [1] cadence/parser.ParseProgram(code, nil)  →  *ast.Program, error
  │
  ├─► [2] trivia.Scan(source)                      →  []trivia.Comment
  │
  ├─► [3] trivia.Attach(program, comments)          →  *trivia.CommentMap
  │
  ├─► [4] rewrite.Apply(program, commentMap, opts)   →  (mutated *ast.Program)
  │       runs: imports → modifiers → parens  (fixed order)
  │
  ├─► [5] render.Program(program, commentMap, opts)  →  prettier.Doc
  │       calls ast.Element.Doc() + comment interleaving
  │
  ├─► [6] prettier.Prettier(writer, doc, lineWidth, indent)  →  formatted []byte
  │
  └─► [7] verify.RoundTrip(source, formatted)        →  error (nil = ok)
```

Steps 1-3 produce the "annotated AST". Steps 4-6 format. Step 7 validates.

---

## Key API surfaces you'll call

### onflow/cadence parser

```go
import "github.com/onflow/cadence/parser"
import "github.com/onflow/cadence/ast"

program, err := parser.ParseProgram(nil, []byte(source), parser.Config{})
```

`*ast.Program` has:
- `.Declarations()` — returns `[]ast.Declaration`
- Each declaration implements `ast.Element` which has:
  - `.Doc() prettier.Doc` — the existing pretty-print representation
  - `.StartPosition()`, `.EndPosition()` — source positions
- Imports are `*ast.ImportDeclaration` with `.Identifiers`, `.Location` (can be `ast.AddressLocation`, `ast.StringLocation`, `ast.IdentifierLocation`)
- Function declarations: `*ast.FunctionDeclaration` with `.Access`, `.Purity`, `.Identifier`, `.ParameterList`, `.ReturnTypeAnnotation`, `.FunctionBlock`
- Composite declarations: `*ast.CompositeDeclaration` with `.CompositeKind`, `.Identifier`, `.Conformances`, `.Members`

Explore the AST types in `go doc` or by reading the cadence source at `github.com/onflow/cadence/ast/`. The `ast.Element` interface and the `ast.Walk`/`ast.Inspector` utilities are essential for comment attachment.

### turbolent/prettier

```go
import "github.com/turbolent/prettier"

// Constructors you'll use:
prettier.Text("keyword")
prettier.Space
prettier.Line{}          // breaks to newline, flattens to space
prettier.SoftLine{}      // breaks to newline, flattens to nothing
prettier.HardLine{}      // always breaks (use for comments)
prettier.Indent{Doc: d}
prettier.Dedent{Doc: d}
prettier.Concat{d1, d2, d3}
prettier.Group{Doc: d}   // try to fit on one line, else break

// Render:
var buf bytes.Buffer
prettier.Prettier(&buf, doc, 100, "    ")
```

---

## Phases

### Phase 1 — Bootstrap (skeleton + naive renderer)

**Goal**: End-to-end plumbing works. Stdin goes in, formatted Cadence comes out (without comments).

**Tasks**:
1. `go mod init github.com/janezpodhostnik/cadencefmt`
2. `go get` the dependencies listed above
3. Create `internal/format/options.go` with `Options` struct and `Default()` function (spec §5.3)
4. Create `internal/format/formatter.go` with `Format(src []byte, filename string, opts Options) ([]byte, error)` that:
   - Parses with `parser.ParseProgram`
   - Calls `program.Doc()` to get a `prettier.Doc`
   - Runs `prettier.Prettier` to render to bytes
   - Returns the bytes
5. Create `cmd/cadencefmt/main.go` that reads stdin, calls `Format`, writes to stdout. No flags beyond `--help` and `--version`.
6. Create the snapshot test harness: a test that walks `testdata/format/*/`, formats `input.cdc`, compares to `golden.cdc`. Support `-update` flag via `go test -update` to regenerate goldens.
7. Write 5 initial snapshot test cases:
   - `hello-world`: simple `access(all) fun main() {}`
   - `variable-declarations`: `let` and `var` with type annotations
   - `simple-resource`: a resource with an init and one function
   - `imports`: a few import statements (no sorting yet)
   - `function-with-params`: a function with multiple parameters

**Exit criteria**:
```bash
go build ./cmd/cadencefmt
echo 'access(all) fun main() {}' | ./cadencefmt  # produces formatted output
go test ./...                                     # all green
```

**Commit and update PROGRESS.md.**

---

### Phase 2 — Comment scanner

**Goal**: Extract all comments from Cadence source bytes, correctly handling nested block comments and string literals.

**Tasks**:
1. Create `internal/format/trivia/comment.go` with types:
   ```go
   type Kind int
   const (
       KindLine Kind = iota     // //
       KindBlock                // /* */
       KindDocLine              // ///
       KindDocBlock             // /** */
   )
   type Comment struct {
       Kind  Kind
       Start ast.Position
       End   ast.Position
       Text  string  // raw text including delimiters
   }
   type CommentGroup struct {
       Comments []Comment
   }
   ```

2. Create `internal/format/trivia/scanner.go` with `func Scan(source []byte) []Comment`. This is a hand-written lexer (~150 lines) that:
   - Walks byte-by-byte through source
   - Tracks string literal state (skip `"..."` content, handle `\"` escapes and `\(expr)` templates with nested paren counting)
   - Detects comment starts: `//`, `/*`
   - For `//`: disambiguate doc-line (`///` where 4th char is not `/`) vs regular line
   - For `/*`: disambiguate doc-block (`/**` where 4th char is not `*` and not `/**/`) vs regular block
   - **Nested block comments**: Cadence supports `/* /* */ */`. Track nesting depth.
   - Records `Comment{Kind, Start, End, Text}` for each

3. Create `internal/format/trivia/group.go` with `func Group(comments []Comment) []*CommentGroup`. Adjacent comments separated only by whitespace (not blank lines) form a group.

4. Write comprehensive table tests in `scanner_test.go`:
   - Basic line comment
   - Basic block comment
   - Doc-line `///`
   - Doc-block `/**`
   - `////` is regular line (not doc-line)
   - `/**/` is regular block (not doc-block)
   - Nested `/* /* */ */`
   - Comment-like sequences inside strings: `"// not a comment"`
   - Comment-like sequences inside string templates: `"\(a /* not a comment */ + b)"`
   - Multiple comments in sequence
   - Mixed comment kinds
   - Empty input
   - Comment at EOF without trailing newline

**Exit criteria**:
```bash
go test ./internal/format/trivia/ -v    # all scanner tests pass
go test ./... 
```

---

### Phase 3 — Comment attachment

**Goal**: Build a CommentMap that binds each comment group to the correct AST node with the correct position class (Leading, Trailing, SameLine, Header, Footer).

**Tasks**:
1. Create `internal/format/trivia/attach.go` with:
   ```go
   type CommentMap struct {
       Header   []*CommentGroup
       Footer   []*CommentGroup
       Leading  map[ast.Element][]*CommentGroup
       Trailing map[ast.Element][]*CommentGroup
       SameLine map[ast.Element]*CommentGroup
   }
   func (cm *CommentMap) Take(n ast.Element) (leading []*CommentGroup, sameLine *CommentGroup, trailing []*CommentGroup)
   func (cm *CommentMap) IsEmpty() bool
   func Attach(program *ast.Program, groups []*CommentGroup, source []byte) *CommentMap
   ```

2. The `Attach` algorithm (spec §6.5-6.6):
   - Walk AST nodes in pre-order (use `ast.Walk` or `ast.Inspector`)
   - Merge-walk with comment groups sorted by position
   - For each group, find the smallest enclosing AST node
   - Classify: Leading, Trailing, or SameLine based on position relative to node's children
   - Disambiguation heuristic for "between two children":
     1. Same-line wins (comment on same line as previous child's end)
     2. Blank line separates (blank line before comment → Leading of next child)
     3. Otherwise → Trailing of previous child
   - Comments before first declaration → Header
   - Comments after last declaration → Footer

3. `Take` removes and returns comments for a node. This is how the renderer ensures each comment is emitted exactly once.

4. Write tests in `attach_test.go` with hand-crafted Cadence source + expected attachments:
   - File header comment
   - File footer comment
   - Leading comment on a function
   - Trailing comment after a function
   - Same-line comment: `let x = 1 // inline`
   - Comment between two declarations (test disambiguation)
   - Doc comment (`///`) above a declaration
   - Comment inside a function body
   - Comment between parameters (if the parser allows this position)
   - Dangling comment in empty block `{ /* comment */ }`

**Exit criteria**:
```bash
go test ./internal/format/trivia/ -v   # all attachment tests pass
go test ./...
```

---

### Phase 4 — Comment interleaving in renderer

**Goal**: Comments appear in formatted output. The formatter now handles comments end-to-end.

**Tasks**:
1. Create `internal/format/render/trivia.go` with:
   ```go
   func wrapWithComments(elem ast.Element, doc prettier.Doc, cm *trivia.CommentMap) prettier.Doc
   ```
   This wraps a node's Doc with its leading comments (before, each followed by `HardLine`), same-line comment (after, preceded by 2 spaces), and trailing comments (after, each preceded by `HardLine`).

2. Create `internal/format/render/render.go` with:
   ```go
   func Program(prog *ast.Program, cm *trivia.CommentMap, opts format.Options) prettier.Doc
   ```
   Walks declarations, calls `wrapWithComments(decl, decl.Doc(), cm)` for each, joins with appropriate blank lines. Emits Header comments before first declaration, Footer after last.

3. Wire comments into the pipeline in `formatter.go`:
   - After parse: `comments := trivia.Scan(source)`
   - Group: `groups := trivia.Group(comments)`
   - Attach: `cm := trivia.Attach(program, groups, source)`
   - Render: `doc := render.Program(program, cm, opts)`
   - After render: assert `cm.IsEmpty()` — if not, return internal error with positions of orphaned comments

4. Update existing snapshot tests to include comments in their golden files.

5. Add new snapshot test cases:
   - `comment-header-footer`: file with header and footer comments
   - `comment-leading`: `// comment` above a declaration
   - `comment-trailing`: comment after a closing brace
   - `comment-sameline`: `let x = 1  // inline comment`
   - `comment-doc-line`: `/// doc comment` above a function
   - `comment-doc-block`: `/** doc block */` above a struct
   - `comment-block-nested`: `/* outer /* inner */ outer */`
   - `comment-between-decls`: comments between two functions

6. Add idempotence test: for each snapshot case, format twice and assert byte-equal.

7. Add comment preservation test: scan comments from input and output, assert multiset equality of comment texts.

**Exit criteria**:
```bash
go test ./...   # all pass, including idempotence and comment preservation
echo 'access(all) fun main() {} // hello' | ./cadencefmt  # comment appears in output
```

---

### Phase 5 — Rewrite passes

**Goal**: Imports are sorted/grouped, modifiers are canonically ordered, redundant parens are stripped.

**Tasks**:
1. Create `internal/format/rewrite/rewrite.go`:
   ```go
   type Rewriter interface {
       Name() string
       Rewrite(prog *ast.Program, cm *trivia.CommentMap) error
   }
   func Apply(prog *ast.Program, cm *trivia.CommentMap, opts format.Options) error
   ```
   `Apply` runs rewriters in fixed order: imports → modifiers → parens.

2. `rewrite/imports.go` (spec §8.3):
   - Group imports: standard (identifier-only) → address (by address then identifier) → string (lexicographic)
   - Sort within groups by identifier
   - Deduplicate
   - Separate groups with blank lines
   - Must update CommentMap if import nodes are reordered (move attached comments with their import)

3. `rewrite/modifiers.go` (spec §8.4):
   - Canonical order: access-modifier → static → native → view → kind-keyword
   - Walk all declarations, reorder modifiers where needed
   - Do NOT rewrite `pub`/`priv` — preserve as-is

4. `rewrite/parens.go` (spec §15, Phase 6):
   - Strip redundant parentheses only where the parse is unambiguous without them
   - Be conservative — if in doubt, keep the parens

5. Wire into pipeline in `formatter.go`, between attach and render.

6. Snapshot tests:
   - `imports-sorting`: unsorted imports → sorted, grouped output
   - `imports-dedup`: duplicate imports collapsed
   - `imports-with-comments`: comments attached to imports survive reordering
   - `modifiers-reorder`: `view access(all) fun` → `access(all) view fun`
   - `modifiers-deprecated`: `pub fun` stays as `pub fun`

**Exit criteria**:
```bash
go test ./...
# Verify imports sorting works:
echo 'import "B"
import "A"
access(all) fun main() {}' | ./cadencefmt
# Output should show import "A" before import "B"
```

---

### Phase 6 — Style overrides in renderer

**Goal**: All formatting rules from spec §8 are implemented. This is the largest phase — ~50 render functions.

**Tasks**:
1. For each AST element kind, compare the output of the existing `elem.Doc()` with the spec's style rules. Where they differ, write a custom `render*` function in the appropriate file (`render/decl.go`, `render/stmt.go`, `render/expr.go`, `render/type.go`).

2. Key style rules to implement (spec §8):
   - **Line width 100**, 4-space indent (configurable via Options)
   - **Function signatures**: single line if fits, else break after `(`, one param per line, **trailing comma in multi-line**
   - **Same rule for**: call expressions, array/dict literals, generic type args
   - **Composite bodies**: opening brace on same line, members separated by blank lines if any spans multiple lines
   - **Pre/post conditions**: one condition per line, indented
   - **Casting**: single space around `as`/`as?`/`as!`, can break before `as`
   - **Resource operators**: `<-`, `<-!` surrounded by single space
   - **Pragmas**: flush-left, top of file, before imports
   - **Transactions**: blocks separated by blank lines if any spans multiple lines
   - **Blank lines**: at most 1 (configurable via `KeepBlankLines`) between declarations
   - **Comments**: line comments trimmed of trailing whitespace, block comments verbatim, doc-block leading-asterisk alignment, same-line separated by 2 spaces

3. The render package structure:
   - `render.go`: `Program()` entry point, dispatches to declaration renderers
   - `decl.go`: functions, composites, events, enums, entitlements, variables, imports, transactions, pragmas
   - `stmt.go`: if/else, switch, for/while, return, assignments, emit, destroy
   - `expr.go`: calls, member access, literals, binary/unary ops, casting, create, ternary
   - `type.go`: optional, reference, array, dict, function types, intersection, capability

4. Write snapshot tests for every declaration kind, every statement kind, every expression kind, every type kind. See spec Appendix A for the complete list. Target ~100+ test cases in this phase.

5. Run idempotence + comment preservation tests on all new cases.

**Exit criteria**:
```bash
go test ./...                        # all pass
go test ./internal/format/... -count 1 -run TestIdempotence  # explicit check
```

The formatter should now produce correct, styled output for all Cadence constructs (without comments inside expressions — those come later as edge cases are discovered).

---

### Phase 7 — CLI polish

**Goal**: All CLI flags from spec §10 are implemented with correct exit codes.

**Tasks**:
1. Add `cobra` CLI with flags (spec §10.2):
   - `-w, --write`: write changes back to files
   - `-c, --check`: exit 1 if any file would change, print paths
   - `-d, --diff`: print unified diff
   - `--config FILE`: explicit config path
   - `--no-config`: ignore config files
   - `--stdin-filename`: filename for diagnostics on stdin
   - `--no-verify`: skip round-trip check
   - `--version`: print version

2. Implement behavior (spec §10.1):
   - No paths + stdin is TTY → print help, exit 2
   - No paths + stdin is pipe → format stdin to stdout
   - Paths → walk recursively for `.cdc` files, format in-place with `-w`

3. Exit codes (spec §10.3): 0 success, 1 check-failed, 2 usage error, 3 parse error, 4 internal error

4. Implement `internal/diff/diff.go` for `--diff` output

5. Implement `internal/config/config.go` and `search.go` for TOML config discovery (spec §9)

**Exit criteria**:
```bash
# Test all modes:
echo 'access(all) fun main(){}' | ./cadencefmt                    # stdout
./cadencefmt -c testdata/format/hello-world/input.cdc              # exit code
./cadencefmt -d testdata/format/hello-world/input.cdc              # diff output  
./cadencefmt --version                                              # version string
go test ./...
```

---

### Phase 8 — Verify pass

**Goal**: Round-trip AST equivalence checking is implemented as a safety net.

**Tasks**:
1. Create `internal/format/verify/verify.go`:
   ```go
   func RoundTrip(original []byte, formatted []byte) error
   ```
   - Parse both `original` and `formatted`
   - Walk both ASTs in parallel
   - Assert every node has same kind and same significant fields (excluding positions)
   - Comments excluded (checked separately by comment preservation tests)

2. Wire into pipeline: runs always in `--check` mode and tests, optional in normal mode (`--no-verify` skips it)

3. Add idempotence + round-trip as part of the standard snapshot test harness (every test case automatically runs both checks)

**Exit criteria**:
```bash
go test ./...
# Intentionally break rendering of one construct, verify that RoundTrip catches it
```

---

### Phase 9 — LSP server

**Goal**: `cadencefmt-lsp` binary works with Neovim and Helix for format-on-save.

**Tasks**:
1. `go get go.lsp.dev/protocol go.lsp.dev/jsonrpc2`
2. Create `internal/lsp/server.go` and `handlers.go` (spec §11)
3. Handle: `initialize`, `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didClose`, `textDocument/formatting`
4. `textDocument/formatting` → call `format.Format()`, return single TextEdit for whole document
5. Config discovery on initialize (walk up from rootUri for `cadencefmt.toml`)
6. Create `cmd/cadencefmt-lsp/main.go`
7. Test manually with an editor

**Exit criteria**:
```bash
go build ./cmd/cadencefmt-lsp
# Manual test: open a .cdc file in an LSP-capable editor, trigger format
go test ./...
```

---

### Phase 10 — Corpus testing and hardening

**Goal**: The formatter handles real-world Cadence contracts without errors.

**Tasks**:
1. Add real-world contracts to `testdata/corpus/` (as git submodules or downloaded files):
   - `onflow/flow-core-contracts`
   - `onflow/flow-ft`
   - `onflow/flow-nft`

2. Write a corpus test that for each `.cdc` file:
   - Formats successfully (no panics, no errors)
   - Is idempotent (`format(format(f)) == format(f)`)
   - Passes round-trip AST check

3. Fix all bugs discovered. This phase is iterative — run corpus, fix, repeat.

4. Add fuzz targets:
   - `FuzzFormat`: arbitrary bytes, assert no panics (parse errors are fine)
   - `FuzzRoundtrip`: valid-parsing bytes only, assert idempotence + round-trip

**Exit criteria**:
```bash
go test ./... 
go test -run TestCorpus ./internal/format/ -v   # all corpus files pass
go test -fuzz FuzzFormat -fuzztime=60s ./internal/format/
```

---

### Phase 11 — Nix flake and CI

**Goal**: Reproducible builds and CI pipeline.

**Tasks**:
1. Create `flake.nix` per spec §4.2 (starter content is in the spec). Run `nix build`, fix `vendorHash`.
2. Create `.github/workflows/ci.yml`:
   - Go matrix: linux/amd64 × Go 1.22 and 1.23 (can narrow from spec's full matrix for practical CI)
   - Steps: checkout, setup-go, build, test
3. Create `LICENSE` (Apache-2.0)
4. Create `README.md` with project status, quick-start for `go install` and CLI usage

**Exit criteria**:
```bash
nix build               # produces binary
nix flake check         # passes
# CI pipeline green on push
```

---

## Style rules quick reference

These are the defaults. See spec §8 for full details. All are configurable via `cadencefmt.toml`.

| Rule | Default |
|------|---------|
| Line width | 100 (soft) |
| Indent | 4 spaces |
| Import ordering | standard → address → string, sorted within group |
| Modifier order | access → static → native → view → kind-keyword |
| Trailing comma | Always in multi-line lists, never in single-line |
| Blank lines | Max 1 between declarations |
| Same-line comment spacing | 2 spaces before `//` |
| Line comment trailing whitespace | Trimmed |
| Block comments | Verbatim, no reflow |
| Doc-block `/**` | Leading-asterisk alignment normalized |
| String templates `\(expr)` | Preserved verbatim |

---

## Comment attachment algorithm quick reference

This is the trickiest part of the codebase. See spec §6 for full details.

**Scanner**: Hand-written lexer over source bytes. Must handle:
- Nested block comments `/* /* */ */` (track depth)
- Skip comment-like sequences inside string literals (track string state, handle `\"` escapes)
- Skip comment-like sequences inside string templates `\(expr)` (track nested paren depth while in string template)
- Disambiguate `///` (doc-line) from `////` (regular line — 4th char is `/`)
- Disambiguate `/**` (doc-block) from `/**/` (regular empty block) and `/***` (regular block)

**Grouping**: Adjacent comments with no blank line between them form a `CommentGroup`.

**Attachment**: Merge-walk of comment groups (sorted by position) and AST nodes (pre-order traversal).
- Find smallest enclosing node for each group
- Classify as Leading, Trailing, or SameLine
- **Disambiguation** (for comments between two sibling nodes):
  1. Same-line as previous sibling's end → SameLine of previous
  2. Blank line between previous sibling and comment → Leading of next
  3. Otherwise → Trailing of previous

**Take()**: Removes and returns comments for a node. After rendering, assert the CommentMap is empty.

---

## PROGRESS.md template

Create this file at repo root on your first session:

```markdown
# cadencefmt Implementation Progress

- [ ] Phase 1: Bootstrap (skeleton + naive renderer)
- [ ] Phase 2: Comment scanner
- [ ] Phase 3: Comment attachment
- [ ] Phase 4: Comment interleaving in renderer
- [ ] Phase 5: Rewrite passes (imports, modifiers, parens)
- [ ] Phase 6: Style overrides in renderer
- [ ] Phase 7: CLI polish
- [ ] Phase 8: Verify pass
- [ ] Phase 9: LSP server
- [ ] Phase 10: Corpus testing and hardening
- [ ] Phase 11: Nix flake and CI
```

---

## Debugging tips

- **"orphaned comments" error**: The CommentMap has comments that no render function called `Take()` for. This means you're missing a `wrapWithComments` call for some AST node type. Use the positions in the error to find which node contains the orphaned comment.
- **Idempotence failure**: Format the output a second time and diff. The difference shows which construct isn't stable. Often caused by trailing whitespace, inconsistent blank line handling, or a Group that breaks differently on re-format.
- **Round-trip failure**: The AST of the formatted output doesn't match the original. Diff the AST dumps. Usually means the renderer is emitting something the parser interprets differently (e.g., operator precedence changes when parens are removed).
- **Comment in wrong position**: Print the CommentMap after attachment. Check that the comment's source position falls within the expected node's range. The disambiguation heuristic (§6.6) is usually the culprit.
- **Exploring the cadence AST**: Use `go doc github.com/onflow/cadence/ast` and read the source. The `ast.Walk` function and `ast.Element` interface are your main tools. Every AST type is in `github.com/onflow/cadence/ast/`.
