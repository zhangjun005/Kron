import { invoke, isTauri } from '@tauri-apps/api/core';
import type { Greeting, ProjectMeta } from '../types/ipc';

/**
 * Type-safe wrapper around Tauri `invoke('kron_*', args)`.
 * Mirrors `dev-docs/design/05-GUI设计.md` § 3.2 IPC contract.
 *
 * Tauri 2 does NOT inject `window.__TAURI__` (that's Tauri v1 only).
 * The canonical check is `isTauri()` from `@tauri-apps/api/core`, which
 * looks for `window.__TAURI_INTERNALS__`. The IPC pipeline requires:
 *   1. CSP: script-src 'unsafe-eval' (set in tauri.conf.json)
 *   2. Running inside a Tauri WebView (not a plain browser)
 *   3. The Rust backend has exposed the command via `#[tauri::command]`
 */

export { isTauri } from '@tauri-apps/api/core';

const ERR_NOT_TAURI =
  'IPC not available: not running inside a Tauri WebView. ' +
  'Launch with `npm run tauri dev` from the project root.';

export const ipc = {
  kronProjectList(): Promise<ProjectMeta[]> {
    if (!isTauri()) {
      return Promise.reject(new Error(ERR_NOT_TAURI));
    }
    return invoke<ProjectMeta[]>('kron_project_list');
  },

  kronGreet(name: string): Promise<Greeting> {
    if (!isTauri()) {
      return Promise.reject(new Error(ERR_NOT_TAURI));
    }
    return invoke<Greeting>('kron_greet', { name });
  },
};
