---
name: ts-style
description: >
  Enforce TypeScript coding standards in Kron. Rules: no any except at three trust boundaries, zod at validation boundaries, no non-null ! assertions, strict tsconfig. Use when writing or reviewing TS/TSX code in this repo.
---

# ts-style

This skill is the authoritative source of truth for TypeScript code in Kron. Apply it when generating, editing, or reviewing `.ts` / `.tsx` files.

## Key reference

- **Google TypeScript Style Guide**: https://google.github.io/styleguide/tsguide.html
- **TypeScript strict mode best practices (2026)**: TypeScript 6.0 (March 2026) made `strict: true` the compiler default — it is no longer an aspiration, it is the baseline.

Core principles: **Safety > Clarity > Simplicity > Maintainability**.

## Hard rules — violate these and the code is wrong

- **No `any`** except at three explicit trust boundaries (see below). Every other use → `unknown` + guard, generic `T`, concrete type, or discriminated union.
- **No `as Type` for narrowing.** Use a type guard or `zod.parse()`. `as const` is fine; `satisfies` is preferred over `as` for shape validation.
- **No non-null `!` assertion** on optional values. Narrow with a guard or explicit throw.
- **No `==` / `!=`.** Always `===` / `!==`.
- **No empty `catch {}`.** Discriminate the error or re-throw.
- **No `eval`** or `Function()` constructor.

## The three allowed `any` boundaries

`any` is permitted only in these specific situations:

1. **Parsing untrusted JSON / YAML** from a file or network (before validation).
2. **Third-party library interop** where the library has no types or wrong types — add `// FIXME: <library> missing types` comment.
3. **Test fixtures and assertions** — explicit `as any` in a `describe` block is fine.

Every other `any` use is a failure.

## Runtime validation with zod

At every trust boundary (file input, network response, user input), validate with zod. The inferred type is the source of truth:

```ts
// ✅
import { z } from 'zod'
const TaskSchema = z.object({
  id: z.string(),
  status: z.enum(['open', 'done']),
  priority: z.enum(['low', 'medium', 'high']),
})
type Task = z.infer<typeof TaskSchema>

async function loadTasks(): Promise<Task[]> {
  const res = await fetch('/api/tasks')
  const data: unknown = await res.json()
  return z.array(TaskSchema).parse(data)  // throws on invalid shape
}

// ❌ any leaks all the way through
async function loadTasks(): Promise<any[]> {
  const res = await fetch('/api/tasks')
  return res.json()
}
```

## Discriminated unions and branded types

Prefer these over optional fields and loose types:

```ts
// ✅ Discriminated union — exhaustively narrowed
type Result<T> =
  | { ok: true; data: T }
  | { ok: false; error: string }

function handle<T>(r: Result<T>) {
  if (r.ok) return r.data
  return r.error
}

// ✅ Branded type — prevents mixing unrelated string IDs
type TaskId = string & { readonly __brand: 'TaskId' }
type ProjectId = string & { readonly __brand: 'ProjectId' }

function loadTask(id: TaskId): Task { ... }
// loadTask("abc" as TaskId)  // OK — intentional
// loadTask(projectId)        // ❌ type error — ProjectId ≠ TaskId
```

## Error handling

Never swallow errors silently:

```ts
// ❌
try {
  await fs.promises.writeFile(path, data)
} catch {}

// ✅ — discriminate and handle or re-throw
try {
  await fs.promises.writeFile(path, data)
} catch (err) {
  if ((err as NodeJS.ErrnoException).code === 'EACCES') {
    throw new Error(`permission denied: ${path}`)
  }
  throw err
}
```

Typed errors over generic `Error`:

```ts
class TaskNotFoundError extends Error {
  readonly code = 'TASK_NOT_FOUND'
  constructor(readonly taskId: string) {
    super(`task not found: ${taskId}`)
  }
}
```

## tsconfig — non-negotiable

TypeScript 6.0 (March 2026) made `strict: true` the compiler default, but these two high-impact flags are **not** in `strict` and must be explicitly set:

```jsonc
{
  "compilerOptions": {
    "strict": true,                      // compiler default since TS 6.0, explicit is fine
    "noUncheckedIndexedAccess": true,     // MUST — adds undefined to every index access
    "exactOptionalPropertyTypes": true,   // MUST — distinguishes absent vs undefined
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "useUnknownInCatchVariables": true,  // catch (e: unknown), not e: any
    "verbatimModuleSyntax": true,         // explicit import/export, no rewrites
    "forceConsistentCasingInFileNames": true,
    "isolatedModules": true,
    "skipLibCheck": true
  }
}
```

If you encounter a build error from one of these settings, fix the code — do not change the tsconfig to silence it.

## Naming

| What | Rule | Example |
|------|------|---------|
| File | kebab-case | `task-store.ts`, `parse-frontmatter.ts` |
| Class / Type / Interface | PascalCase, no `I` prefix | `TaskStore`, `TaskFrontmatter` |
| Function / variable | camelCase | `loadTask`, `taskContent` |
| Boolean | `is` / `has` / `can` prefix | `isOpen`, `hasChildren` |
| Constant | UPPER_SNAKE | `MAX_TASKS_PER_FILE` |
| React component | PascalCase, file matches | `TaskList.tsx` |

## File organization

```
src/
  components/     ← React components
  lib/            ← Pure utility functions, shared logic
  api/            ← Thin HTTP client over the Go backend
  types/          ← Shared domain types and zod schemas
  main.tsx        ← Entry point
index.ts          ← Re-exports public surface only; no implementation
```

Imports ordered:
1. Node built-ins (`node:fs`, `node:path`)
2. External packages (`react`, `zod`)
3. Internal aliases (`@/types`, `@/lib`)
4. Relative imports

Use `import type { Task } from './types'` for type-only imports.

## Comments

- JSDoc on every exported function and type.
- Inline comments explain *why*, not *what*.
- `// FIXME: <desc>` for known issues with a ticket/issue URL if available.
- `// TODO: <desc>` for planned work.

## Anti-patterns

```ts
// ❌ Non-null ! on find()
const task = tasks.find(t => t.id === id)!
console.log(task.title)  // runtime crash if not found

// ✅
const task = tasks.find(t => t.id === id)
if (!task) throw new TaskNotFoundError(id)
console.log(task.title)

// ❌ == instead of ===
if (status == 'open') { ... }

// ✅
if (status === 'open') { ... }

// ❌ hasOwnProperty on object
if (obj.hasOwnProperty('id')) { ... }

// ✅
if (Object.hasOwn(obj, 'id')) { ... }  // Node 16.9+

// ❌ Type assertion for shape validation
const task = raw as Task

// ✅ satisfies keeps narrow inferred type while validating
const task = TaskSchema.parse(raw)  // zod
// or
const task = raw satisfies Task  // built-in
```

## What this skill doesn't cover

- Frontend framework choice (React, Vue, Svelte) — document when the UI phase starts.
- CSS / styling approach — not yet decided.
- State management (Zustand, Jotai, etc.) — defer until UI phase.
- API client implementation details beyond "thin HTTP wrapper over Go backend".
