# CLAUDE.md - Guidelines for Code in plexctl

## Build Commands
- Install: `go install ./cmd/plexctl`
- Lint: `golangci-lint run`
- Test all: `go test ./...`
- Test single: `go test ./path/to/package -run TestName`
- Test with coverage: `go test -coverprofile=coverage.out ./...`
- View coverage: `go tool cover -html=coverage.out`

## Code Style
- Format: `gofmt -s -w .` (simplify code)
- Imports: Use `goimports` with local prefix `github.com/nathanleclaire/plexctl`
- Line length: Keep functions under 60 lines, 40 statements
- Complexity: Max cyclomatic complexity 10, max nesting 3
- Error handling: Return errors (don't panic), check `errcheck` linter
- Naming: Use Go conventions (CamelCase for exported, camelCase for private)
- Comments: Document all exported functions, types, and constants
- Context: Pass `context.Context` as first parameter in long-running functions
- Testing: Use testify framework, implement parallel tests when possible
- Security: Follow guidelines from gosec linter (G101-G607)
