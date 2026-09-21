---
name: ts-style
description: Enforces TypeScript coding standards for the Kron project — strict type safety, no `any` except at trust boundaries, idiomatic error handling, and project-specific conventions. Use when writing, editing, or reviewing TypeScript or TSX files in this repository.
---

# TypeScript Style — Kron Project

Rules that every `.ts` / `.tsx` file in this repo must follow. Apply automatically when generating or modifying TypeScript code.

## 1. Type safety

### `any` is forbidden except at three explicit trust boundaries
- Parsing untrusted JSON / YAML from a file or network
- Third-party library interop where types are missing or wrong (must have `// FIXME: <library>` comment)
- Test fixtures and assertions

Every other use of `any` must be replaced with:
- A concrete type
- `unknown` (paired with a type guard)
- A generic `T`
- A discriminated union

### Prefer `unknown` over `any` for unknown shapes

```ts
// ✅ unknown forces a type guard
function parseTask(input: unknown): Task {
  if (!isTask(input)) throw new Error("invalid task")
  return input
}

// ❌ any silently bypasses type checking
function parseTask(input: any): Task {
  return input as Task
}
```

### Type guards: write your own, or use `zod`

- For runtime-validated boundaries (user input, file contents, network responses), use `zod` schemas. The inferred type is the source of truth.
- For internal narrowing, write a small `function isX(x: unknown): x is X`.

### `as` is a code smell

- `as Type` for **casting** is only allowed when the type narrowing cannot be expressed in the language (rare). Prefer a guard.
- `as const` is encouraged for literals and tuples.

### Avoid non-null assertions `!`

- `x!.foo` is almost always wrong. Either narrow with a guard or check explicitly:
  ```ts
  // ❌
  const task = tasks.find(t => t.id === id)!
  // ✅
  const task = tasks.find(t => t.id === id)
  if (!task) throw new Error(`task ${id} not found`)
  ```

## 2. Error handling

### Never swallow errors silently
```ts
// ❌
try { await fs.promises.writeFile(p, data) } catch {}

// ✅
try {
  await fs.promises.writeFile(p, data)
} catch (err) {
  if ((err as NodeJS.ErrnoException).code !== 'ENOENT') throw err
}
```

### Typed errors over generic `Error`
- Define error classes when callers need to discriminate: `class TaskNotFoundError extends Error { code = 'TASK_NOT_FOUND' as const }`.
- For async boundaries, propagate via `Promise.reject` or `throw`.

## 3. Naming

| Element | Convention | Example |
|---------|-----------|---------|
| File | kebab-case | `task-store.ts`, `parse-frontmatter.ts` |
| Class / Type | PascalCase | `TaskStore`, `TaskFrontmatter` |
| Function / variable | camelCase | `loadTask`, `taskContent` |
| Constant | UPPER_SNAKE | `MAX_TASKS_PER_FILE` |
| React component | PascalCase, file matches name | `TaskList.tsx` |
| Interface | no `I` prefix | `Task`, not `ITask` |
| Boolean | `is`/`has`/`can` prefix | `isOpen`, `hasChildren` |

## 4. File organization

- One exported symbol per file when reasonable; helper exports go in the same file.
- `index.ts` re-exports the public surface only.
- Imports ordered: built-ins → external → internal (`@/...`) → relative.
- Use `type` import for type-only imports: `import type { Task } from './model'`.

## 5. Project layout (TypeScript subprojects)

```
kron-frontend/           ← any UI work lives here
├── src/
│   ├── components/
│   ├── lib/
│   ├── api/             ← thin client over Go HTTP API
│   └── main.tsx
├── tsconfig.json        ← strict: true, noUncheckedIndexedAccess: true
├── package.json
└── vite.config.ts       ← or next.config, etc.
```

### `tsconfig.json` must have:
```jsonc
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "exactOptionalPropertyTypes": true,
    "noFallthroughCasesInSwitch": true
  }
}
```

## 6. Comments and JSDoc

- JSDoc on every exported function or type (mirrors Go's godoc requirement).
- Inline comments explain **why**, not what.
- `// FIXME:` for known issues, `// TODO:` for planned work, both with author and date when relevant.

## 7. Commit messages

Same format as Go:
```
<scope>: <imperative summary>
```

- `feat(api): add client for kron task CRUD`
- `fix(ui): clamp long task titles in TaskList`
- `chore: bump typescript to 5.6`

## 8. Forbidden patterns

- `as any` casts — always `unknown` + guard
- Non-null `!` assertions on optional values
- `eval`, `Function()` constructor
- `Object.prototype.hasOwnProperty.call` is fine; `obj.hasOwnProperty(...)` is not (unsafe)
- `==` and `!=` — always `===` / `!==`

## Examples

For side-by-side good/bad examples covering all rules above, see [examples.md](examples.md).
