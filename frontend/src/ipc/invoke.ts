import { invoke } from '@tauri-apps/api/core';
import type { Greeting, ProjectMeta } from '../types/ipc';

/**
 * Type-safe wrapper around Tauri `invoke('kron_*', args)`.
 * Mirrors `dev-docs/design/05-GUI设计.md` § 3.2 IPC contract.
 *
 * Tauri IPC requires:
 *   1. CSP: script-src must include 'unsafe-eval' or 'unsafe-inline'
 *   2. Dev mode: window.__TAURI__ injected by the WebView
 *   3. Frontend dist must exist (checked by tauri.conf.json frontendDist)
 */

/** True when running inside a Tauri WebView (not a plain browser). */
export function isTauri(): boolean {
  return typeof window !== 'undefined' && '__TAURI__' in window;
}

export const ipc = {
  /**
   * Guard: rejects if called outside a Tauri WebView.
   * The Home component checks `isTauri()` and shows a clear error.
   */
  kronProjectList(): Promise<ProjectMeta[]> {
    if (!isTauri()) {
      return Promise.reject(
        new Error(
          'IPC not available: window.__TAURI__ not found. ' +
          'Is the frontend served from Tauri, not a plain browser?'
        )
      );
    }
    return invoke<ProjectMeta[]>('kron_project_list');
  },

  kronGreet(name: string): Promise<Greeting> {
    if (!isTauri()) {
      return Promise.reject(
        new Error(
          'IPC not available: window.__TAURI__ not found. ' +
          'Is the frontend served from Tauri, not a plain browser?'
        )
      );
    }
    return invoke<Greeting>('kron_greet', { name });
  },
};
