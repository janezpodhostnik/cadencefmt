# Contributing to cadencefmt

## Development Setup

### Prerequisites

[Nix](https://nixos.org/) is recommended (pins the exact Go version and tools). Alternatively, install Go 1.26+ and [golangci-lint](https://golangci-lint.run/) manually.

### Setup

```bash
git clone https://github.com/janezpodhostnik/cadencefmt.git
cd cadencefmt

# With Nix + direnv (recommended)
direnv allow    # auto-loads the dev shell from flake.nix

# Or manually
nix develop

# Fetch corpus test data
git submodule update --init
```

### Common Tasks

```
just build          # build both binaries
just test           # run all tests (fast)
just corpus         # run corpus tests against real-world contracts
just lint           # golangci-lint
just fuzz           # fuzz for 60s per target
just update-golden  # refresh golden files after intentional changes
just snapshot NAME  # run a single snapshot test
just check          # build + test + lint
```

## Making Changes

1. Fork the repo and create a feature branch from `main`
2. Keep PRs focused -- one logical change per PR
3. For bug fixes and new formatting rules, add a snapshot test case:
   - Create `testdata/format/<name>/input.cdc` with unformatted input
   - Run `just update-golden` to generate `golden.cdc`
   - Verify the golden output looks correct

## Commit Messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

Format: `<type>(<scope>): <description>`

**Types:**

| Type | Use for |
|------|---------|
| `fix` | Bug fixes (formatter output, comment handling, etc.) |
| `feat` | New formatting rules or CLI features |
| `refactor` | Code changes that don't affect behavior |
| `test` | Adding or updating tests |
| `docs` | Documentation changes |
| `ci` | CI/CD configuration |
| `chore` | Dependencies, tooling, config |

**Scope** is optional but encouraged. Use the package name: `trivia`, `render`, `rewrite`, `verify`, `lsp`, `cli`.

**Examples:**

```
fix(trivia): handle nested block comments inside string templates
feat(render): add line break before long function parameter lists
test: add snapshot case for nil-coalescing in return statements
docs: update README installation instructions
ci: switch to nix flake for environment setup
chore: bump onflow/cadence to v1.11.0
```

## Testing

Run `just check` before submitting a PR. This builds, tests, and lints.

Four invariants must hold for every input -- CI enforces these:

1. **Round-trip correctness** -- `parse(format(S))` structurally equals `parse(S)`
2. **Idempotence** -- `format(format(S)) == format(S)` byte-for-byte
3. **Comment preservation** -- every comment appears exactly once
4. **Fail-safe** -- parse errors produce a non-zero exit, nothing written to stdout

### Adding a Test Case

```bash
mkdir testdata/format/my-new-case
# Write the unformatted input
cat > testdata/format/my-new-case/input.cdc << 'EOF'
access(all) fun   example()  {  }
EOF
# Generate the golden file
just update-golden
# Verify it looks right
cat testdata/format/my-new-case/golden.cdc
```

### Corpus Tests

Run `just corpus` to test against real-world contracts from [flow-core-contracts](https://github.com/onflow/flow-core-contracts). These require the git submodule to be checked out (`git submodule update --init`).

## Code Guidelines

- All packages live under `internal/` -- there is no public Go API
- Do not fork or modify the `onflow/cadence` parser -- use it as a library
- Do not add new IR primitives to `turbolent/prettier` -- use the existing algebra
- Rewrite pass order in `internal/format/rewrite/` is fixed -- do not reorder without bumping `CurrentFormatVersion` in `options.go`

## VS Code Extension Development

The thin TypeScript wrapper lives at [`editors/vscode/`](editors/vscode/). It spawns `cadencefmt-lsp` via `vscode-languageclient`. Node.js 22+ is required (or use `nix develop`, which provides it).

```bash
just vscode-build       # npm ci + typecheck + build (esbuild bundle)
just vscode-package     # builds the .vsix
just vscode-install     # builds and installs into your local VS Code
```

## Releases

Distribution is via [GitHub Releases](https://github.com/janezpodhostnik/cadencefmt/releases) only — no marketplace. Each release ships:

- `cadencefmt` and `cadencefmt-lsp` binaries cross-compiled for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}` (Windows not built).
- `cadencefmt.vsix` — the VS Code extension.

Cutting a release:

1. Push a tag `vX.Y.Z` (e.g. `git tag v0.1.1 && git push origin v0.1.1`).
2. CI (`.github/workflows/release.yml`) builds all artifacts and creates the GitHub Release with auto-generated notes. No tokens or secrets needed.

The git tag is the single source of truth for the version. CI stamps the tag into `editors/vscode/package.json` at build time (so the `.vsix` manifest gets the right version) and injects it into the Go binaries via `-ldflags -X main.version=…`. The committed `package.json` value is just a placeholder for local development; do not bump it manually.

The extension's auto-update check polls the GitHub Releases API at most once per day and prompts users to download a new version when one is available.

## Reporting Issues

- Include the input `.cdc` source (or a minimal reproduction)
- Show expected vs actual formatted output
- File issues at [github.com/janezpodhostnik/cadencefmt/issues](https://github.com/janezpodhostnik/cadencefmt/issues)
