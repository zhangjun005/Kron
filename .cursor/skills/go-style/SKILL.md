---
name: go-style
description: >
  Enforce Go coding standards in Kron. Rules: no any/interface{} escape hatches, always wrap errors with context, use errors.Is/As not ==, one concrete rule per bullet, concrete examples. Use when writing or reviewing Go code in this repo.
---

# go-style

This skill is the authoritative source of truth for Go code in Kron. Apply it when generating, editing, or reviewing `.go` files. If a rule conflicts with existing code, surface the conflict — do not silently ignore it.

## Key references

- **Google Go Style Guide** (canonical): https://google.github.io/styleguide/go/guide.html
- **Go Code Review Comments** (normative supplement): https://tip.golang.org/wiki/CodeReviewComments
- **Effective Go** — not actively updated for generics/modules; use for core idioms only

Core principles (in priority order): **Clarity > Simplicity > Concision > Maintainability > Consistency**.

## Hard rules — violate these and the code is wrong

- **No `any` / `interface{}` in function signatures.** Use generics or concrete types. Allowed only for `encoding/json` / `encoding/xml` interop — if you reach for it, add a `// why` comment.
- **No bare type assertion `x.(T)`.** Must use the comma-ok form: `v, ok := x.(T)`.
- **No `reflect` package** without a `// why: <specific reason>` comment.
- **No `time.Sleep` in production code.** Use channels or `sync.WaitGroup`.
- **No `==` / `!=` for error comparison.** Use `errors.Is` / `errors.As`.
- **No discarded error (`_ =`) unless the function is documented safely ignorable** (e.g., idempotent `Close`).

## Error handling — one way, done right

Every error must be wrapped at the point of origin:

```go
// ✅ wrap with operation context
data, err := os.ReadFile(path)
if err != nil {
    return nil, fmt.Errorf("reading intent file %s: %w", path, err)
}

// ❌ bare return — caller has no idea what failed
data, err := os.ReadFile(path)
if err != nil {
    return nil, err
}
```

Define sentinel errors for known failure modes:

```go
var ErrIntentNotFound = errors.New("intent not found")
```

Discriminate with `errors.Is` / `errors.As`, never `==`:

```go
// ✅
if errors.Is(err, ErrIntentNotFound) { ... }
var pathErr *fs.PathError
if errors.As(err, &pathErr) { ... }

// ❌
if err == ErrIntentNotFound { ... }
```

## Type safety — prefer the specific over the general

Generics over `interface{}`:

```go
// ✅
func Map[T, U any](s []T, f func(T) U) []U { ... }

// ❌
func Map(s []interface{}, f func(interface{}) interface{}) []interface{} { ... }
```

Pointer vs value receivers — pick one per type, stay consistent:
- Pointer: type has mutable fields, a `Lock()` / `Sync()` method, or is larger than a pointer.
- Value: small, immutable, or a map/chan/func.
- **Never mix** on the same type.

## Naming

| What | Rule | Example |
|------|------|---------|
| Package | lowercase, one word, no underscores | `store`, `model` |
| Exported | PascalCase | `LoadIntent`, `IntentStore` |
| Unexported | camelCase | `parseFrontmatter` |
| Acronyms | all caps | `HTTPClient`, `ParseJSON` |
| Constants | PascalCase | `MaxIntentsPerFile = 1000` |
| File | lowercase, `_` for multi-word | `intent_store.go` |

Never prefix an interface with `I` (no `IIntentStore`). Name it what it does: `IntentReader`, `Store`.

## Project layout

```
cmd/kron/
├── main.go      ← thin wiring: dispatch subcommands, no business logic
├── cli/         ← access layer: cobra commands (kron init / add / lint)
└── serve-mcp/   ← access layer: MCP stdio server (planned)

internal/        ← bottom layer packages only
├── model/       ← domain types (Intent, Config), zero external deps
├── store/       ← file I/O, YAML frontmatter read/write
└── parser/      ← CLI argument parsing, markdown body extraction
```

Business logic goes in `internal/` (bottom layer) or in access-layer subcommands under
`cmd/kron/<sub>/`. `internal/` MUST NOT import any package under `cmd/kron/`.
Access-layer subcommands under `cmd/kron/<sub>/` MUST NOT import each other. If two
access layers need to share behavior, lift the shared code into `internal/` first.

No circular imports between `internal/*` packages.

## Documentation

Every exported identifier needs a godoc comment. Format:

```go
// LoadIntent reads and parses an intent file from .kron/intents/.
// Returns ErrIntentNotFound if the file does not exist.
func LoadIntent(id string) (*Intent, error) { ... }
```

Rules:
- First sentence starts with the identifier name.
- Package comment on every package.

## Formatting

Run `gofmt` before every commit. There is no fixed line length — if a line feels too long, refactor it instead.

```bash
gofmt -w .
```

Use `go vet ./...` and `go test ./...` before committing.

## Modernizing existing code

Go 1.26+ includes `go fix` which automatically modernizes code toward current idioms. Run it to modernize a package:

```bash
go fix ./internal/store
```

## Commit messages

Format: `<scope>: <imperative summary>`

```
feat(store): add frontmatter parser for intent files
fix(cli): handle missing .kron/ directory on kron ls
docs: clarify storage format in README
chore: bump github.com/spf13/cobra to 1.8.0
```

Body explains *why*, not *what*. Reference design docs: `docs/abstractDesign/XX-name.md`.

## Things that need a `// why` comment

- Any use of `reflect`
- `//go:generate` directives
- `// nolint:` suppressions
- `time.Sleep` (even in test setup — document the sleep reason)
- Global mutable state
- `init()` functions that set environment variables

## Anti-patterns

```go
// ❌ Stutter: package name repeats in exported symbol
package intentstore
func (s *IntentStore) LoadIntent(...)  // callers: intentstore.LoadIntent

// ✅ Clean: package is the noun, symbol says the verb
package store
func (s *Store) LoadIntent(...)

// ❌ Mixed-case acronym
func parseJson(b []byte)  // ✅ HTTP, JSON, URL — all caps

// ❌ Business logic in cmd/
// ✅ cmd/kron/main.go: if err := cli.Execute(); err != nil { os.Exit(1) }
```

## Pre-implementation discussion (apply before writing any code)

Before writing code for a new exported function, type, or package, **propose the approach as a short written plan and wait for approval**. This is not optional.

A plan should cover:
- What the code does (one sentence)
- What types and functions are being added, and why their shapes are chosen that way
- Where it lives (`internal/model/`, `internal/store/`, etc.) and why
- How it is tested
- What the error model looks like (sentinel errors, wrapped errors)

When the plan is simple (e.g., adding one test, adding a trivial helper), a one-sentence confirmation in chat is enough. When it touches a new type, changes a public API, or adds a new file, write a full plan.

> Rationale: architectural decisions compound. A wrong type shape or function name in `internal/model` forces breaking changes across every caller. Catching it before writing code costs 2 minutes of chat; catching it after costs an hour of migration.

## What this skill doesn't cover

- Go toolchain setup. Handle separately.
- Test naming conventions — follow the standard library convention.
- Race detector usage (`go test -race`) — enabled for integration tests, not unit tests by default.
