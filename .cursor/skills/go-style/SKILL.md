---
name: go-style
description: Enforces Go coding standards for the Kron project — strict type safety, no `any`-equivalent escapes, idiomatic error handling, and project-specific layout. Use when writing, editing, or reviewing Go code in this repository.
---

# Go Style — Kron Project

Rules that every Go file in this repo must follow. Apply automatically when generating or modifying `.go` files.

## 1. Type safety

### Never use these escape hatches
- **`any`** — only acceptable when interoperating with `encoding/json` or `encoding/xml` for genuinely free-form data. Prefer concrete types or `interface{ Validate() error }`.
- **`interface{}`** — same rule as `any`. Prefer generics, concrete types, or constrained type parameters.
- **Type assertions without the comma-ok form**: `v, ok := x.(T)` is required; `v := x.(T)` panics on mismatch.
- **`reflect`** — not allowed unless the alternative is genuinely impossible. Document the reason if used.

### Prefer generics over `interface{}` or `any`

```go
// ✅ Use generics for container operations
func Map[T, U any](s []T, f func(T) U) []U { ... }

// ❌ Don't use interface{} in signatures
func Map(s []interface{}, f func(interface{}) interface{}) []interface{} { ... }
```

### Pointer vs value receivers — pick one, stay consistent
- Value receivers: when the type is small, immutable, or a map/chan/func.
- Pointer receivers: when the type has any mutable field or has a `Sync()`/`Lock()` method.
- **Never mix** — pick pointer or value for the whole type.

## 2. Error handling

### Every error must be handled or explicitly ignored
- **Wrap with context**: `fmt.Errorf("loading task %s: %w", id, err)`
- **Never use `panic`** in library code. CLI `main()` may panic only on programmer errors (unrecoverable).
- **Never discard**: `_ = someFunc()` is allowed only when the return value is documented to be safely ignorable (e.g., `defer f.Close()`).
- **Sentinel errors** for known cases: `var ErrTaskNotFound = errors.New("task not found")`.

### Use `errors.Is` / `errors.As`, not `==`

```go
// ✅
if errors.Is(err, ErrTaskNotFound) { ... }
var pathErr *fs.PathError
if errors.As(err, &pathErr) { ... }

// ❌
if err == ErrTaskNotFound { ... }
```

## 3. Naming

| Element | Convention | Example |
|---------|-----------|---------|
| Package | lowercase, single word, no underscores | `store`, `model`, `cli` |
| Exported | PascalCase | `TaskStore`, `LoadTask` |
| Unexported | camelCase | `parseFrontmatter` |
| Constant | PascalCase (not SCREAMING_SNAKE) | `MaxTasksPerFile = 1000` |
| Acronyms | all caps or all lower | `HTTPClient`, `httpClient` (not `HttpClient`) |
| Interface | noun or "-er" suffix | `Task`, `TaskReader` |

### File naming
- One type per file when reasonable: `task.go` defines `type Task struct`.
- Test files: `task_test.go` in the same package.
- Lowercase, underscores for multi-word: `frontmatter.go`, `task_store.go`.

## 4. Project layout

```
kron/
├── cmd/
│   └── kron/
│       └── main.go         ← CLI entry, thin
├── internal/
│   ├── model/              ← domain types (Task, Project)
│   ├── store/              ← file I/O, frontmatter parsing
│   └── cli/                ← cobra/urfave-cli command implementations
├── go.mod
└── go.sum
```

- **Business logic goes in `internal/`** — never in `cmd/`.
- **`cmd/kron/main.go` is wiring only** — parse flags, dispatch to `internal/cli/`, exit.
- **No circular imports** between `internal/*` packages.

## 5. Documentation

### Package comment (required for every package)
```go
// Package store reads and writes Kron task files on disk.
//
// All functions are safe for concurrent use.
package store
```

### Exported identifiers: godoc-style comment
```go
// LoadTask reads and parses a task file from .kron/tasks/.
// Returns ErrTaskNotFound if the file does not exist.
func LoadTask(id string) (*Task, error) { ... }
```

- Comment starts with the identifier name: `// LoadTask reads...` not `// This function reads...`.
- One sentence on the first line; further detail on subsequent lines.

## 6. Commit messages

Format: `<scope>: <imperative summary>`

- `feat(store): add frontmatter parser for task files`
- `fix(cli): handle missing .kron/ directory on `kron ls``
- `docs: clarify storage format in README`
- `chore: bump github.com/spf13/cobra to 1.8.0`
- `refactor(model): split Task into Create/Update structs`

Body explains *why*, not *what*. Reference design docs with `dev-docs/design/XX-name.md` if applicable.

## 7. Things that always need a `// why` comment

- Reflection use
- `go:generate` directives
- `// nolint:` suppressions
- `time.Sleep` (always replace with channel/waitgroup in production code)
- Global mutable state

## Examples

For side-by-side good/bad examples covering all rules above, see [examples.md](examples.md).
