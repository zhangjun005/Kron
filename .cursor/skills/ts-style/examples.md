# TypeScript Style Examples

Side-by-side good/bad examples. Every bad example is a real failure mode we want to prevent in the Kron codebase.

## Type safety

### `any` at the wrong boundary

```ts
// ❌ Bad: any leaks into the call site
async function loadTasks(): Promise<any> {
  const res = await fetch('/api/tasks')
  return res.json()
}

// ✅ Good: zod validates at the boundary, returns a precise type
import { z } from 'zod'
const TaskSchema = z.object({
  id: z.string(),
  status: z.enum(['open', 'done']),
})

async function loadTasks(): Promise<Task[]> {
  const res = await fetch('/api/tasks')
  const data: unknown = await res.json()
  return z.array(TaskSchema).parse(data)
}
```

### `as` cast without narrowing

```ts
// ❌ Bad: trust the source, get runtime errors
const task = (await res.json()) as Task

// ✅ Good: validate then narrow
const data: unknown = await res.json()
const task = TaskSchema.parse(data)
```

### Non-null assertion on `find()`

```ts
// ❌ Bad: undefined at runtime crashes the app
const task = tasks.find(t => t.id === id)!
console.log(task.title)

// ✅ Good: explicit handle
const task = tasks.find(t => t.id === id)
if (!task) throw new TaskNotFoundError(id)
console.log(task.title)
```

## Error handling

### Empty catch

```ts
// ❌ Bad: silent failure
try {
  await fs.promises.writeFile(path, data)
} catch {}

// ✅ Good: discriminate, then re-throw or recover
try {
  await fs.promises.writeFile(path, data)
} catch (err) {
  if ((err as NodeJS.ErrnoException).code === 'EACCES') {
    throw new Error(`permission denied: ${path}`)
  }
  throw err
}
```

### String error matching

```ts
// ❌ Bad: brittle, breaks on message changes
if (err.message.includes('not found')) { ... }

// ✅ Good: typed errors
if (err instanceof TaskNotFoundError) { ... }
```

## Naming

### `I` prefix on interfaces

```ts
// ❌ Bad: Hungarian-style noise
interface ITask { ... }
interface IUserStore { ... }

// ✅ Good: just the noun
interface Task { ... }
interface UserStore { ... }
```

### Boolean naming

```ts
// ❌ Bad: ambiguous
const open = true
const task = getTask()

// ✅ Good: is/has/can prefix
const isOpen = true
const task = getTask()
```

## Project layout

### Public surface scattered

```ts
// ❌ Bad: importing internals from a sibling file
import { parseInternalFormat } from './_internal/parser'

// ✅ Good: index.ts re-exports the public surface only
// src/lib/index.ts
export { loadTask } from './load-task'
export { TaskSchema } from './schema'
```

### Imports not ordered

```ts
// ❌ Bad: random order, hard to scan
import { TaskSchema } from './schema'
import React from 'react'
import type { Task } from './types'
import fs from 'node:fs'

// ✅ Good: built-ins → external → internal → relative, types grouped
import fs from 'node:fs'

import React from 'react'

import type { Task } from './types'
import { TaskSchema } from './schema'
```

## Forbidden patterns

### `==` and `!=`

```ts
// ❌ Bad: coerces types — can hide bugs
if (status == 'open') { ... }
if (count != 0) { ... }

// ✅ Good: strict equality, no surprises
if (status === 'open') { ... }
if (count !== 0) { ... }
```

### `obj.hasOwnProperty(...)` directly

```ts
// ❌ Bad: fails if obj has a `hasOwnProperty` property
if (obj.hasOwnProperty('id')) { ... }

// ✅ Good: protected from prototype pollution
if (Object.prototype.hasOwnProperty.call(obj, 'id')) { ... }

// ✅ Best: use Object.hasOwn (Node 16.9+)
if (Object.hasOwn(obj, 'id')) { ... }
```
