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

# Run all checks (build, test, lint)
check: build test lint
