# Default recipe: list available commands
default:
    @just --list

# Build both binaries
build:
    go build ./cmd/cadencefmt
    go build ./cmd/cadencefmt-lsp

# Run all tests (excludes corpus; use `just corpus` for that)
test:
    go test -short ./...

# Run tests for a specific package (e.g., just test-pkg ./internal/format/trivia/)
test-pkg pkg:
    go test {{ pkg }} -v

# Format Go source code
fmt:
    go fmt ./...

# Run golangci-lint
lint:
    golangci-lint run ./...

# Run fuzz tests (default 60s per target)
fuzz duration="60s":
    go test -fuzz FuzzFormat -fuzztime={{ duration }} -run '^$' ./internal/format/
    go test -fuzz FuzzRoundtrip -fuzztime={{ duration }} -run '^$' ./internal/format/

# Update golden test files
update-golden:
    go test ./internal/format/... -update

# Run a specific snapshot test by name
snapshot name:
    go test ./internal/format/... -run "TestSnapshot/{{ name }}" -v

# Run corpus tests (requires: git submodule update --init)
corpus:
    go test ./internal/format/ -run TestCorpus -v

# Run benchmarks (snapshot inputs only, no submodule needed)
bench:
    go test -bench=BenchmarkFormat -benchmem -count=3 -run='^$' ./internal/format/

# Run all benchmarks including corpus and per-stage breakdown
bench-all:
    go test -bench=. -benchmem -count=3 -run='^$' ./internal/format/

# Run per-stage breakdown benchmarks only
bench-stages:
    go test -bench=BenchmarkStage -benchmem -count=3 -run='^$' ./internal/format/

# Update vendorHash in flake.nix (run after changing go.mod)
update-vendor-hash:
    #!/usr/bin/env bash
    set -euo pipefail
    # Use a fake hash to force nix to compute the real one
    sed -i 's|vendorHash = ".*|vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";|' flake.nix
    real_hash=$(nix build .#cadencefmt 2>&1 | grep 'got:' | awk '{print $2}') || true
    if [ -z "$real_hash" ]; then
        echo "nix build succeeded — vendorHash is already correct"
        git checkout flake.nix
    else
        sed -i "s|vendorHash = \".*|vendorHash = \"${real_hash}\"; # update with: just update-vendor-hash|" flake.nix
        echo "Updated vendorHash to ${real_hash}"
    fi

# Run all checks (build, test, lint)
check: build test lint
