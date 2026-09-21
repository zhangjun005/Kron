# Go Style Examples

Side-by-side good/bad examples. Every bad example is a real failure mode we want to prevent in the Kron codebase.

## Type safety

### `any` / `interface{}` in function signatures

```go
// ❌ Bad: signature loses all type information
func ParseTasks(data interface{}) ([]interface{}, error) { ... }

// ✅ Good: generic with a meaningful constraint
func ParseTasks[T TaskLike](data []T) ([]T, error) { ... }
```

### Type assertion without comma-ok

```go
// ❌ Bad: panics if .metadata is nil
md := task.Metadata["key"].(string)

// ✅ Good: handle the type mismatch explicitly
v, ok := task.Metadata["key"].(string)
if !ok {
    return fmt.Errorf("expected string, got %T", task.Metadata["key"])
}
```

### Reflection for what's just a generic

```go
// ❌ Bad: reflection for "sum a slice"
func Sum(values interface{}) float64 {
    v := reflect.ValueOf(values)
    // ... 20 lines of reflection
}

// ✅ Good: generic, type-checked at compile time
func Sum[T ~float64 | ~int](values []T) T {
    var s T
    for _, v := range values {
        s += v
    }
    return s
}
```

## Error handling

### Discarding errors

```go
// ❌ Bad: silent data loss
_ = os.Remove(tmpFile)

// ✅ Bad: panic on programmer error is acceptable; silent drop is not
defer func() {
    if err := os.Remove(tmpFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
        log.Printf("cleanup tmp: %v", err)
    }
}()
```

### Wrapping without context

```go
// ❌ Bad: caller has no idea which file failed
data, err := os.ReadFile(path)
if err != nil {
    return err
}

// ✅ Good: include the path and operation
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("reading task file %s: %w", path, err)
}
```

### Direct error comparison

```go
// ❌ Bad: doesn't work for wrapped errors
if err == ErrTaskNotFound { ... }

// ✅ Good: unwraps correctly
if errors.Is(err, ErrTaskNotFound) { ... }
```

## Naming

### Mixed-case acronyms

```go
// ❌ Bad: Http, Json, Url
type HttpClient struct { ... }
func ParseJson(b []byte) (T, error) { ... }

// ✅ Good: HTTP, JSON, URL
type HTTPClient struct { ... }
func ParseJSON(b []byte) (T, error) { ... }
```

### Stutter

```go
// ❌ Bad: package name + symbol
package taskstore
func taskstore.Load(id string) { ... }
// callers: taskstore.Load — redundant

// ✅ Good: package is the noun, symbol says the verb
package store
func store.LoadTask(id string) { ... }
// callers: store.LoadTask — clean
```

## Project layout

### Business logic in `cmd/`

```go
// ❌ Bad: cmd/kron/main.go doing real work
package main
func main() {
    data, _ := os.ReadFile(".kron/tasks/foo.md")
    // ... 80 lines of parsing logic ...
}

// ✅ Good: cmd is wiring, logic is in internal
// cmd/kron/main.go
func main() {
    if err := cli.Execute(); err != nil {
        os.Exit(1)
    }
}

// internal/store/load.go
func LoadTask(id string) (*Task, error) { ... }
```

## Things that always need a `// why` comment

```go
// ❌ Bad: invisible magic
func init() {
    os.Setenv("TZ", "UTC")
}

// ✅ Good: explain why
// init: forces UTC so task timestamps in .kron/ files are
// timezone-agnostic — the frontmatter spec requires UTC.
func init() {
    os.Setenv("TZ", "UTC")
}
```
